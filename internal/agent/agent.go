package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	metrics "github.com/DaniilPodruchnyi/metrics-collector/internal/proto"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/retry"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/security"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/workerpool"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// MetricValue — структура для хранения значения метрики
type MetricValue struct {
	Type    string
	Gauge   float64
	Counter int64
}

// Agent представляет агент сбора метрик
type Agent struct {
	config     *config.AgentConfig
	metrics    map[string]*MetricValue
	mu         sync.RWMutex
	client     *http.Client
	workerPool *workerpool.WorkerPool
	wg         sync.WaitGroup
	publicKey  *rsa.PublicKey
	localIP    string
	grpcConn   *grpc.ClientConn
	grpcClient metrics.MetricsClient
}

// New создает новый агент с конфигурацией
func New(cfg *config.AgentConfig) *Agent {
	metricValues := make(map[string]*MetricValue)

	// Инициализируем runtime метрики
	runtimeGaugeMetrics := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc",
		"HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased", "HeapSys",
		"LastGC", "Lookups", "MCacheInuse", "MCacheSys", "MSpanInuse",
		"MSpanSys", "Mallocs", "NextGC", "NumForcedGC", "NumGC", "OtherSys",
		"PauseTotalNs", "StackInuse", "StackSys", "Sys", "TotalAlloc",
	}

	for _, name := range runtimeGaugeMetrics {
		metricValues[name] = &MetricValue{Type: "gauge"}
	}

	// Специальные метрики
	metricValues["PollCount"] = &MetricValue{Type: "counter", Counter: 0}
	metricValues["RandomValue"] = &MetricValue{Type: "gauge", Gauge: rand.Float64()}

	// Gopsutil метрики (инициализация)
	metricValues["TotalMemory"] = &MetricValue{Type: "gauge"}
	metricValues["FreeMemory"] = &MetricValue{Type: "gauge"}

	agent := &Agent{
		config:  cfg,
		metrics: metricValues,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Загружаем публичный ключ для асимметричного шифрования, если указан путь
	if cfg.HasCryptoKeyPath() {
		pub, err := security.LoadPublicKeyFromFile(cfg.CryptoKeyPath)
		if err != nil {
			log.Printf("Failed to load public key from %s: %v. Asymmetric encryption disabled.", cfg.CryptoKeyPath, err)
		} else {
			agent.publicKey = pub
			log.Printf("Asymmetric encryption enabled with public key: %s", cfg.CryptoKeyPath)
		}
	}

	// Создаем worker pool с обработчиком метрик
	poolConfig := workerpool.Config{
		Workers:     cfg.RateLimit,
		BufferSize:  100,
		RetryConfig: retry.DefaultConfig(),
	}

	agent.workerPool = workerpool.New(poolConfig, agent.metricJobHandler)

	// Определяем IP-адрес хоста агента (best-effort)
	if ip := detectLocalIP(); ip != "" {
		agent.localIP = ip
		log.Printf("Detected local IP for X-Real-IP: %s", ip)
	} else {
		log.Printf("Could not reliably detect local IP for X-Real-IP header")
	}

	// Инициализируем gRPC‑клиент, если задан адрес gRPC‑сервера.
	if cfg.GRPCAddress != "" {
		// Используем NewClient вместо устаревшего DialContext.
		conn, err := grpc.NewClient(
			cfg.GRPCAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.ForceCodec(metrics.JSONCodec)),
		)
		if err != nil {
			log.Printf("Failed to create gRPC client for %s: %v. Falling back to HTTP transport.", cfg.GRPCAddress, err)
		} else {
			agent.grpcConn = conn
			agent.grpcClient = metrics.NewMetricsClient(conn)
			log.Printf("gRPC transport enabled, server: %s", cfg.GRPCAddress)
		}
	}

	return agent
}

// Run запускает основной цикл агента с несколькими горутинами
func (a *Agent) Run(ctx context.Context) {
	log.Println("Agent started")
	if a.config.HasKey() {
		log.Println("Request signing enabled")
	}
	if a.publicKey != nil {
		log.Println("Asymmetric encryption of requests enabled")
	}
	log.Printf("Worker pool size: %d", a.config.RateLimit)

	// 1. Запускаем worker pool только для HTTP‑транспорта.
	if a.grpcClient == nil {
		a.workerPool.Start(ctx)
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

	// Ждем завершения коллекторов
	a.wg.Wait()

	// Останавливаем worker pool, если он запускался.
	if a.grpcClient == nil {
		a.workerPool.Stop()
	}

	log.Println("Agent stopped gracefully")
}

// Shutdown корректно останавливает агента
func (a *Agent) Shutdown(ctx context.Context) error {
	log.Println("Initiating agent shutdown...")

	// Даем время на завершение текущих операций
	done := make(chan struct{})

	go func() {
		// Ждем завершения коллекторов
		a.wg.Wait()

		// Останавливаем worker pool только один раз (для HTTP‑транспорта)
		if a.grpcClient == nil {
			a.workerPool.Stop()
		}

		close(done)
	}()

	select {
	case <-done:
		log.Println("Agent shutdown completed")

		if a.grpcConn != nil {
			_ = a.grpcConn.Close()
		}

		return nil
	case <-ctx.Done():
		log.Println("Agent shutdown timeout exceeded")
		return ctx.Err()
	}
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

// metricsReporter отправляет метрики в worker pool каждые ReportInterval
func (a *Agent) metricsReporter(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.ReportInterval)
	defer ticker.Stop()

	log.Println("Metrics reporter started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Metrics reporter stopped")
			return
		case <-ticker.C:
			if a.grpcClient != nil {
				log.Println("Sending metrics batch via gRPC...")
				if err := a.sendMetricsBatchGRPC(ctx); err != nil {
					log.Printf("Failed to send metrics via gRPC: %v", err)
				}
			} else {
				log.Println("Publishing metrics to worker pool...")
				a.publishMetrics()
			}
		}
	}
}

// sendMetricsBatchGRPC формирует батч метрик и отправляет его на gRPC‑сервер.
func (a *Agent) sendMetricsBatchGRPC(ctx context.Context) error {
	a.mu.RLock()

	metricsCopy := make([]model.Metrics, 0, len(a.metrics))
	for name, metric := range a.metrics {
		m := model.Metrics{
			ID:    name,
			MType: metric.Type,
		}

		switch metric.Type {
		case "counter":
			delta := metric.Counter
			m.Delta = &delta
		case "gauge":
			value := metric.Gauge
			m.Value = &value
		}

		metricsCopy = append(metricsCopy, m)
	}

	a.mu.RUnlock()

	if len(metricsCopy) == 0 {
		return nil
	}

	req := &metrics.UpdateMetricsRequest{
		Metrics: make([]*metrics.Metric, 0, len(metricsCopy)),
	}

	for _, m := range metricsCopy {
		pm := &metrics.Metric{
			ID: m.ID,
		}

		if m.MType == model.Counter && m.Delta != nil {
			pm.Type = metrics.MetricCOUNTER
			pm.Delta = *m.Delta
		} else if m.MType == model.Gauge && m.Value != nil {
			pm.Type = metrics.MetricGAUGE
			pm.Value = *m.Value
		}

		req.Metrics = append(req.Metrics, pm)
	}

	callCtx := ctx
	if a.localIP != "" {
		md := metadata.Pairs("x-real-ip", a.localIP)
		callCtx = metadata.NewOutgoingContext(ctx, md)
	}

	_, err := a.grpcClient.UpdateMetrics(callCtx, req)
	return err
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

	// Собираем информацию о CPU с интервалом 100ms для точности
	if percentages, err := cpu.Percent(100*time.Millisecond, true); err == nil {
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

// publishMetrics отправляет все метрики в worker pool
func (a *Agent) publishMetrics() {
	a.mu.RLock()

	// Создаем копии метрик для безопасной отправки
	metricsCopy := make([]model.Metrics, 0, len(a.metrics))

	for name, metric := range a.metrics {
		m := model.Metrics{
			ID:    name,
			MType: metric.Type,
		}

		switch metric.Type {
		case "counter":
			// Копируем значение
			delta := metric.Counter
			m.Delta = &delta
		case "gauge":
			// Копируем значение
			value := metric.Gauge
			m.Value = &value
		}

		metricsCopy = append(metricsCopy, m)
	}

	a.mu.RUnlock()

	// Теперь безопасно отправляем копии в worker pool
	count := 0
	skipped := 0

	for _, m := range metricsCopy {
		if a.workerPool.Submit(m) {
			count++
		} else {
			skipped++
		}
	}

	if skipped > 0 {
		log.Printf("Warning: %d metrics skipped (worker pool queue full)", skipped)
	}
	log.Printf("Published %d metrics to worker pool", count)
}

// metricJobHandler обрабатывает задачу отправки метрики (используется в WorkerPool)
func (a *Agent) metricJobHandler(ctx context.Context, job workerpool.Job) error {
	metric, ok := job.(model.Metrics)
	if !ok {
		return fmt.Errorf("invalid job type: expected model.Metrics")
	}

	return a.sendSingleMetric(ctx, metric)
}

// sendSingleMetric отправляет одну метрику на сервер
func (a *Agent) sendSingleMetric(ctx context.Context, metric model.Metrics) error {
	url := a.config.GetServerURL() + "/update"

	// Сериализуем в JSON
	plain, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}

	// Подписываем НЕСЖАТЫЕ данные
	var hash string
	if a.config.HasKey() {
		hash = security.ComputeHMAC(plain, a.config.Key)
	}

	var bodyReader io.Reader

	// Если настроен публичный ключ — шифруем запрос и НЕ используем gzip
	var encrypted bool
	if a.publicKey != nil {
		cipher, err := security.EncryptRSA(plain, a.publicKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt payload: %w", err)
		}
		bodyReader = bytes.NewReader(cipher)
		encrypted = true
	} else {
		// Без асимметричного шифрования — поведение как раньше: gzip
		var gzipBuf bytes.Buffer
		gz := gzip.NewWriter(&gzipBuf)
		if _, err := gz.Write(plain); err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
		if err := gz.Close(); err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}
		bodyReader = &gzipBuf
	}

	// Создаем запрос с контекстом
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if encrypted {
		// Тело полностью зашифровано, gzip не используется
		req.Header.Set("X-Encrypted", "rsa")
	} else {
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")
	}
	req.Header.Set("User-Agent", "metrics-agent/3.0")

	// Добавляем X-Real-IP с IP-адресом хоста агента (если определен)
	if a.localIP != "" {
		req.Header.Set("X-Real-IP", a.localIP)
	}

	// Добавляем хеш
	if a.config.HasKey() {
		req.Header.Set("HashSHA256", hash)
	}

	// Отправляем с учетом контекста
	resp, err := a.client.Do(req)
	if err != nil {
		// Проверяем, не был ли запрос отменен
		if ctx.Err() != nil {
			return fmt.Errorf("request cancelled: %w", ctx.Err())
		}
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

// detectLocalIP пытается определить IP-адрес хоста агента.
// Возвращает первый найденный не-loopback IPv4 адрес либо пустую строку.
func detectLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range ifaces {
		// Пропускаем неактивные и loopback-интерфейсы
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil {
				continue
			}

			ip = ip.To4()
			if ip == nil {
				// Не IPv4
				continue
			}

			if !ip.IsLoopback() {
				return ip.String()
			}
		}
	}

	return ""
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
