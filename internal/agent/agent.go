package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/retry"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/security"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// MetricValue — структура для хранения значения метрики
type MetricValue struct {
	Type    string
	Gauge   float64
	Counter int64
}

// Agent представляет агент сбора метрик
type Agent struct {
	config  *config.AgentConfig
	metrics map[string]*MetricValue
	mu      sync.RWMutex // Мьютекс для thread-safe доступа к метрикам
	client  *http.Client
	jobs    chan model.Metrics // Канал для worker pool
	wg      sync.WaitGroup     // WaitGroup для graceful shutdown
}

// New создает новый агент с конфигурацией
func New(cfg *config.AgentConfig) *Agent {
	metrics := make(map[string]*MetricValue)

	// Инициализируем runtime метрики
	runtimeGaugeMetrics := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc",
		"HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased", "HeapSys",
		"LastGC", "Lookups", "MCacheInuse", "MCacheSys", "MSpanInuse",
		"MSpanSys", "Mallocs", "NextGC", "NumForcedGC", "NumGC", "OtherSys",
		"PauseTotalNs", "StackInuse", "StackSys", "Sys", "TotalAlloc",
	}

	for _, name := range runtimeGaugeMetrics {
		metrics[name] = &MetricValue{Type: "gauge"}
	}

	// Специальные метрики
	metrics["PollCount"] = &MetricValue{Type: "counter", Counter: 0}
	metrics["RandomValue"] = &MetricValue{Type: "gauge", Gauge: rand.Float64()}

	// Gopsutil метрики (инициализация)
	metrics["TotalMemory"] = &MetricValue{Type: "gauge"}
	metrics["FreeMemory"] = &MetricValue{Type: "gauge"}

	// CPU метрики будут добавлены динамически

	return &Agent{
		config:  cfg,
		metrics: metrics,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		jobs: make(chan model.Metrics, 100), // Буферизованный канал
	}
}

// Run запускает основной цикл агента с несколькими горутинами
func (a *Agent) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("Agent started")
	if a.config.HasKey() {
		log.Println("Request signing enabled")
	}
	log.Printf("Worker pool size: %d", a.config.RateLimit)

	// 1. Запускаем worker pool (N воркеров)
	for i := 0; i < a.config.RateLimit; i++ {
		a.wg.Add(1)
		go a.worker(ctx, i+1)
	}

	// 2. Запускаем сборщик runtime метрик
	a.wg.Add(1)
	go a.runtimeCollector(ctx)

	// 3. Запускаем сборщик gopsutil метрик
	a.wg.Add(1)
	go a.gopsutilCollector(ctx)

	// 4. Запускаем отправщик метрик (producer)
	a.wg.Add(1)
	go a.metricsReporter(ctx)

	// Ждем завершения всех горутин
	a.wg.Wait()
	log.Println("Agent stopped")
}

// runtimeCollector собирает runtime метрики каждые PollInterval
func (a *Agent) runtimeCollector(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.PollInterval)
	defer ticker.Stop()

	log.Println("Runtime collector started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Runtime collector stopped")
			return
		case <-ticker.C:
			a.collectRuntimeMetrics()
		}
	}
}

// gopsutilCollector собирает метрики через gopsutil каждые PollInterval
func (a *Agent) gopsutilCollector(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.PollInterval)
	defer ticker.Stop()

	log.Println("Gopsutil collector started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Gopsutil collector stopped")
			return
		case <-ticker.C:
			a.collectGopsutilMetrics()
		}
	}
}

// metricsReporter отправляет метрики в канал jobs каждые ReportInterval
func (a *Agent) metricsReporter(ctx context.Context) {
	defer a.wg.Done()
	defer close(a.jobs) // Закрываем канал при выходе

	ticker := time.NewTicker(a.config.ReportInterval)
	defer ticker.Stop()

	log.Println("Metrics reporter started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Metrics reporter stopped")
			return
		case <-ticker.C:
			log.Println("Sending metrics to worker pool...")
			a.publishMetrics()
		}
	}
}

// worker обрабатывает метрики из канала jobs
func (a *Agent) worker(ctx context.Context, id int) {
	defer a.wg.Done()

	log.Printf("Worker %d started", id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopped", id)
			return
		case metric, ok := <-a.jobs:
			if !ok {
				log.Printf("Worker %d: channel closed", id)
				return
			}

			// Отправляем метрику с retry
			retryCfg := retry.DefaultConfig()
			err := retry.Do(func() error {
				return a.sendSingleMetric(metric)
			}, retryCfg)

			if err != nil {
				log.Printf("Worker %d: failed to send metric %s: %v", id, metric.ID, err)
			} else {
				log.Printf("Worker %d: successfully sent metric %s", id, metric.ID)
			}
		}
	}
}

// collectRuntimeMetrics собирает метрики из runtime
func (a *Agent) collectRuntimeMetrics() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	a.mu.Lock()
	defer a.mu.Unlock()

	// Обновляем счётчик опросов
	a.metrics["PollCount"].Counter++

	// Обновляем случайное значение
	a.metrics["RandomValue"].Gauge = rand.Float64()

	// Обновляем все runtime-метрики
	a.metrics["Alloc"].Gauge = float64(ms.Alloc)
	a.metrics["BuckHashSys"].Gauge = float64(ms.BuckHashSys)
	a.metrics["Frees"].Gauge = float64(ms.Frees)
	a.metrics["GCCPUFraction"].Gauge = ms.GCCPUFraction
	a.metrics["GCSys"].Gauge = float64(ms.GCSys)
	a.metrics["HeapAlloc"].Gauge = float64(ms.HeapAlloc)
	a.metrics["HeapIdle"].Gauge = float64(ms.HeapIdle)
	a.metrics["HeapInuse"].Gauge = float64(ms.HeapInuse)
	a.metrics["HeapObjects"].Gauge = float64(ms.HeapObjects)
	a.metrics["HeapReleased"].Gauge = float64(ms.HeapReleased)
	a.metrics["HeapSys"].Gauge = float64(ms.HeapSys)
	a.metrics["LastGC"].Gauge = float64(ms.LastGC)
	a.metrics["Lookups"].Gauge = float64(ms.Lookups)
	a.metrics["MCacheInuse"].Gauge = float64(ms.MCacheInuse)
	a.metrics["MCacheSys"].Gauge = float64(ms.MCacheSys)
	a.metrics["MSpanInuse"].Gauge = float64(ms.MSpanInuse)
	a.metrics["MSpanSys"].Gauge = float64(ms.MSpanSys)
	a.metrics["Mallocs"].Gauge = float64(ms.Mallocs)
	a.metrics["NextGC"].Gauge = float64(ms.NextGC)
	a.metrics["NumForcedGC"].Gauge = float64(ms.NumForcedGC)
	a.metrics["NumGC"].Gauge = float64(ms.NumGC)
	a.metrics["OtherSys"].Gauge = float64(ms.OtherSys)
	a.metrics["PauseTotalNs"].Gauge = float64(ms.PauseTotalNs)
	a.metrics["StackInuse"].Gauge = float64(ms.StackInuse)
	a.metrics["StackSys"].Gauge = float64(ms.StackSys)
	a.metrics["Sys"].Gauge = float64(ms.Sys)
	a.metrics["TotalAlloc"].Gauge = float64(ms.TotalAlloc)
}

// collectGopsutilMetrics собирает метрики через gopsutil
func (a *Agent) collectGopsutilMetrics() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Собираем информацию о памяти
	if v, err := mem.VirtualMemory(); err == nil {
		a.metrics["TotalMemory"].Gauge = float64(v.Total)
		a.metrics["FreeMemory"].Gauge = float64(v.Free)
	} else {
		log.Printf("Failed to get memory info: %v", err)
	}

	// Собираем информацию о CPU (утилизация каждого ядра)
	if percentages, err := cpu.Percent(0, true); err == nil {
		for i, percent := range percentages {
			metricName := fmt.Sprintf("CPUutilization%d", i+1)

			// Инициализируем метрику если её нет
			if _, exists := a.metrics[metricName]; !exists {
				a.metrics[metricName] = &MetricValue{Type: "gauge"}
			}

			a.metrics[metricName].Gauge = percent
		}
	} else {
		log.Printf("Failed to get CPU info: %v", err)
	}
}

// publishMetrics отправляет все метрики в канал jobs
func (a *Agent) publishMetrics() {
	a.mu.RLock()
	defer a.mu.RUnlock()

	count := 0
	for name, metric := range a.metrics {
		m := model.Metrics{
			ID:    name,
			MType: metric.Type,
		}

		switch metric.Type {
		case "counter":
			m.Delta = &metric.Counter
		case "gauge":
			m.Value = &metric.Gauge
		}

		// Отправляем в канал (non-blocking)
		select {
		case a.jobs <- m:
			count++
		default:
			log.Printf("Warning: jobs channel is full, skipping metric %s", name)
		}
	}

	log.Printf("Published %d metrics to worker pool", count)
}

// sendSingleMetric отправляет одну метрику на сервер
func (a *Agent) sendSingleMetric(metric model.Metrics) error {
	url := a.config.GetServerURL() + "/update"

	// Сериализуем в JSON
	buf, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}

	// Подписываем НЕСЖАТЫЕ данные
	var hash string
	if a.config.HasKey() {
		hash = security.ComputeHMAC(buf, a.config.Key)
	}

	// Сжимаем данные gzip
	var gzipBuf bytes.Buffer
	gz := gzip.NewWriter(&gzipBuf)
	if _, err := gz.Write(buf); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Создаем запрос
	req, err := http.NewRequest(http.MethodPost, url, &gzipBuf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", "metrics-agent/3.0")

	// Добавляем хеш
	if a.config.HasKey() {
		req.Header.Set("HashSHA256", hash)
	}

	// Отправляем
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server responded with status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Проверяем подпись ответа
	if a.config.HasKey() {
		receivedHash := resp.Header.Get("HashSHA256")
		if receivedHash != "" {
			if !security.VerifyHMAC(responseBody, a.config.Key, receivedHash) {
				return fmt.Errorf("response signature verification failed")
			}
		}
	}

	return nil
}

// GetMetrics возвращает копию текущих метрик (для тестирования)
func (a *Agent) GetMetrics() map[string]*MetricValue {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]*MetricValue)
	for name, metric := range a.metrics {
		result[name] = &MetricValue{
			Type:    metric.Type,
			Gauge:   metric.Gauge,
			Counter: metric.Counter,
		}
	}
	return result
}

// GetMetric возвращает значение конкретной метрики (для тестирования)
func (a *Agent) GetMetric(name string) (*MetricValue, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	metric, exists := a.metrics[name]
	if !exists {
		return nil, false
	}
	return &MetricValue{
		Type:    metric.Type,
		Gauge:   metric.Gauge,
		Counter: metric.Counter,
	}, true
}
