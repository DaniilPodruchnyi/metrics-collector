package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/retry"
)

// Список gauge-метрик из runtime
var runtimeGaugeMetrics = []string{
	"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc",
	"HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased", "HeapSys",
	"LastGC", "Lookups", "MCacheInuse", "MCacheSys", "MSpanInuse",
	"MSpanSys", "Mallocs", "NextGC", "NumForcedGC", "NumGC", "OtherSys",
	"PauseTotalNs", "StackInuse", "StackSys", "Sys", "TotalAlloc",
}

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
	client  *http.Client
}

// New создает новый агент с конфигурацией
func New(cfg *config.AgentConfig) *Agent {
	metrics := make(map[string]*MetricValue)

	// Инициализируем все gauge-метрики из runtime
	for _, name := range runtimeGaugeMetrics {
		metrics[name] = &MetricValue{Type: "gauge"}
	}

	// Добавляем специальные метрики
	metrics["PollCount"] = &MetricValue{Type: "counter", Counter: 0}
	metrics["RandomValue"] = &MetricValue{Type: "gauge", Gauge: rand.Float64()}

	return &Agent{
		config:  cfg,
		metrics: metrics,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Run запускает основной цикл агента
func (a *Agent) Run() {
	pollTicker := time.NewTicker(a.config.PollInterval)
	reportTicker := time.NewTicker(a.config.ReportInterval)
	defer pollTicker.Stop()
	defer reportTicker.Stop()

	log.Println("Agent started")

	for {
		select {
		case <-pollTicker.C:
			a.collectMetrics()

		case <-reportTicker.C:
			log.Println("Sending metrics to server...")
			a.sendMetrics()
		}
	}
}

// collectMetrics собирает метрики из runtime
func (a *Agent) collectMetrics() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// Обновляем счётчик опросов
	a.metrics["PollCount"].Counter++

	// Обновляем случайное значение
	a.metrics["RandomValue"].Gauge = rand.Float64()

	// Обновляем все runtime-метрики
	a.updateRuntimeMetrics(&ms)
}

// updateRuntimeMetrics обновляет все метрики из runtime.MemStats
func (a *Agent) updateRuntimeMetrics(ms *runtime.MemStats) {
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

// isHTTPRetryable проверяет, является ли HTTP статус код retriable
func isHTTPRetryable(statusCode int) bool {
	// 5xx - server errors (retriable)
	if statusCode >= 500 && statusCode < 600 {
		return true
	}

	// 429 - Too Many Requests (retriable)
	if statusCode == 429 {
		return true
	}

	// 408 - Request Timeout (retriable)
	if statusCode == 408 {
		return true
	}

	// 4xx - client errors (NOT retriable, кроме указанных выше)
	// 2xx, 3xx - успех и редиректы (NOT retriable)
	return false
}

// isRetryableError проверяет, является ли ошибка retriable
func (a *Agent) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Проверяем HTTP статус коды
	// Формат ошибки: "server responded with status XXX"
	var statusCode int
	n, _ := fmt.Sscanf(errStr, "server responded with status %d", &statusCode)
	if n == 1 {
		return isHTTPRetryable(statusCode)
	}

	// Проверяем сетевые ошибки через retry utility
	return retry.IsRetryable(err)
}

// sendMetrics отправляет все метрики на сервер батчем с retry
func (a *Agent) sendMetrics() {
	// Собираем все метрики в batch
	batch := make([]model.Metrics, 0, len(a.metrics))

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

		batch = append(batch, m)
	}

	// Не отправляем пустые батчи
	if len(batch) == 0 {
		log.Println("No metrics to send")
		return
	}

	// Конфигурация retry
	retryCfg := retry.DefaultConfig()

	// Пытаемся отправить batch с retry
	err := retry.Do(func() error {
		err := a.sendMetricsBatch(batch)

		// Проверяем, стоит ли делать retry
		if err != nil && !a.isRetryableError(err) {
			log.Printf("Non-retriable error, skipping retry: %v", err)
			return nil // Возвращаем nil чтобы остановить retry
		}

		return err
	}, retryCfg)

	if err != nil {
		log.Printf("Failed to send metrics batch after %d attempts: %v", retryCfg.MaxAttempts+1, err)

		// Fallback: отправляем по одной (для обратной совместимости)
		log.Println("Falling back to single metric sending...")
		successCount := 0
		errorCount := 0

		for name, metric := range a.metrics {
			// Retry для каждой метрики
			err := retry.Do(func() error {
				err := a.sendMetric(name, metric)

				// Проверяем retriable
				if err != nil && !a.isRetryableError(err) {
					log.Printf("Non-retriable error for metric %s, skipping retry: %v", name, err)
					return nil // Останавливаем retry для non-retriable
				}

				return err
			}, retryCfg)

			if err != nil {
				log.Printf("Failed to send metric %s after retries: %v", name, err)
				errorCount++
			} else {
				successCount++
			}
		}

		log.Printf("Fallback complete: %d success, %d errors", successCount, errorCount)
	} else {
		log.Printf("Successfully sent batch of %d metrics", len(batch))
	}
}

// sendMetricsBatch отправляет метрики батчем на /updates
func (a *Agent) sendMetricsBatch(metrics []model.Metrics) error {
	url := a.config.GetServerURL() + "/updates"

	// Сериализуем в JSON
	buf, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
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
	req.Header.Set("User-Agent", "metrics-agent/2.0")

	// Отправляем
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server responded with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// sendMetric отправляет одну метрику на сервер (fallback для обратной совместимости)
func (a *Agent) sendMetric(name string, metric *MetricValue) error {
	url := a.config.GetServerURL() + "/update"

	var jsonMetric struct {
		ID    string   `json:"id"`
		MType string   `json:"type"`
		Delta *int64   `json:"delta,omitempty"`
		Value *float64 `json:"value,omitempty"`
	}
	jsonMetric.ID = name
	jsonMetric.MType = metric.Type
	switch metric.Type {
	case "counter":
		jsonMetric.Delta = &metric.Counter
	case "gauge":
		jsonMetric.Value = &metric.Gauge
	}

	buf, err := json.Marshal(jsonMetric)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "metrics-agent/2.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server responded with status %d", resp.StatusCode)
	}

	return nil
}

// buildMetricURL строит URL для отправки метрики (legacy)
func (a *Agent) buildMetricURL(name string, metric *MetricValue) string {
	switch metric.Type {
	case "gauge":
		valueStr := strconv.FormatFloat(metric.Gauge, 'g', -1, 64)
		return fmt.Sprintf("%s/update/gauge/%s/%s", a.config.GetServerURL(), name, valueStr)
	case "counter":
		return fmt.Sprintf("%s/update/counter/%s/%d", a.config.GetServerURL(), name, metric.Counter)
	default:
		return ""
	}
}

// GetMetrics возвращает копию текущих метрик (для тестирования)
func (a *Agent) GetMetrics() map[string]*MetricValue {
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

// SendMetricJSON отправляет метрику на сервер в формате JSON с gzip сжатием
func SendMetricJSON(serverURL string, metric model.Metrics) error {
	data, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, serverURL+"/update", &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendMetricsJSONBatch отправляет несколько метрик батчем на /updates
func SendMetricsJSONBatch(serverURL string, metrics []model.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics batch: %w", err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, serverURL+"/updates", &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
