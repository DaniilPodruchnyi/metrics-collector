package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
)

func TestNewAgent(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      3,
	}

	agent := New(cfg)

	if agent.config != cfg {
		t.Error("Agent config not set correctly")
	}
	if agent.client == nil {
		t.Error("HTTP client not initialized")
	}
	if len(agent.metrics) == 0 {
		t.Error("Metrics not initialized")
	}
	if pollCount, exists := agent.metrics["PollCount"]; !exists || pollCount.Type != "counter" {
		t.Error("PollCount metric not initialized correctly")
	}
	if randomValue, exists := agent.metrics["RandomValue"]; !exists || randomValue.Type != "gauge" {
		t.Error("RandomValue metric not initialized correctly")
	}
	if alloc, exists := agent.metrics["Alloc"]; !exists || alloc.Type != "gauge" {
		t.Error("Runtime metrics not initialized correctly")
	}

	// Проверяем gopsutil метрики
	if totalMem, exists := agent.metrics["TotalMemory"]; !exists || totalMem.Type != "gauge" {
		t.Error("TotalMemory metric not initialized correctly")
	}
	if freeMem, exists := agent.metrics["FreeMemory"]; !exists || freeMem.Type != "gauge" {
		t.Error("FreeMemory metric not initialized correctly")
	}

	// Проверяем инициализацию канала
	if agent.jobs == nil {
		t.Error("Jobs channel not initialized")
	}
}

func TestCollectRuntimeMetrics(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      3,
	}

	agent := New(cfg)

	// Получаем начальное значение PollCount
	agent.mu.RLock()
	initialPollCount := agent.metrics["PollCount"].Counter
	agent.mu.RUnlock()

	// Собираем метрики
	agent.collectRuntimeMetrics()

	agent.mu.RLock()
	defer agent.mu.RUnlock()

	if agent.metrics["PollCount"].Counter != initialPollCount+1 {
		t.Errorf("PollCount should be incremented, got %d, want %d",
			agent.metrics["PollCount"].Counter, initialPollCount+1)
	}
	if agent.metrics["Alloc"].Gauge == 0 {
		t.Error("Runtime metrics should be updated")
	}
	if agent.metrics["RandomValue"].Type != "gauge" {
		t.Error("RandomValue should be gauge type")
	}
}

func TestCollectGopsutilMetrics(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      3,
	}

	agent := New(cfg)

	// Собираем gopsutil метрики
	agent.collectGopsutilMetrics()

	agent.mu.RLock()
	defer agent.mu.RUnlock()

	// Проверяем, что метрики памяти собраны
	if agent.metrics["TotalMemory"].Gauge == 0 {
		t.Error("TotalMemory should be updated")
	}
	if agent.metrics["FreeMemory"].Gauge == 0 {
		t.Error("FreeMemory should be updated")
	}

	// Проверяем, что собрана хотя бы одна CPU метрика
	cpuFound := false
	for name := range agent.metrics {
		if strings.HasPrefix(name, "CPUutilization") {
			cpuFound = true
			break
		}
	}
	if !cpuFound {
		t.Error("CPU utilization metrics should be collected")
	}
}

func TestSendSingleMetric(t *testing.T) {
	// Тестовый HTTP сервер, который принимает JSON POST и проверяет заголовки
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		userAgent := r.Header.Get("User-Agent")
		if !strings.Contains(userAgent, "metrics-agent") {
			t.Errorf("Expected User-Agent to contain metrics-agent, got %s", userAgent)
		}

		// Проверяем Content-Encoding
		contentEncoding := r.Header.Get("Content-Encoding")
		if contentEncoding != "gzip" {
			t.Errorf("Expected Content-Encoding gzip, got %s", contentEncoding)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.AgentConfig{
		ServerAddress:  server.URL,
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      3,
	}

	agent := New(cfg)
	agent.collectRuntimeMetrics()

	// Создаем тестовую метрику
	value := 42.0
	metric := model.Metrics{
		ID:    "TestMetric",
		MType: "gauge",
		Value: &value,
	}

	if err := agent.sendSingleMetric(metric); err != nil {
		t.Errorf("sendSingleMetric() error = %v", err)
	}
}

func TestSendSingleMetricError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.AgentConfig{
		ServerAddress:  server.URL,
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      3,
	}

	agent := New(cfg)

	value := 123.45
	metric := model.Metrics{
		ID:    "TestGauge",
		MType: "gauge",
		Value: &value,
	}

	err := agent.sendSingleMetric(metric)
	if err == nil {
		t.Error("sendSingleMetric() should return error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Error should mention status code 500, got: %v", err)
	}
}

func TestPublishMetrics(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      3,
	}

	agent := New(cfg)
	agent.collectRuntimeMetrics()

	// Публикуем метрики в канал
	go agent.publishMetrics()

	// Читаем из канала
	timeout := time.After(2 * time.Second)
	metricsReceived := 0

	for {
		select {
		case metric, ok := <-agent.jobs:
			if !ok {
				t.Error("Jobs channel closed unexpectedly")
				return
			}
			metricsReceived++

			// Проверяем структуру метрики
			if metric.ID == "" {
				t.Error("Metric ID should not be empty")
			}
			if metric.MType != "gauge" && metric.MType != "counter" {
				t.Errorf("Invalid metric type: %s", metric.MType)
			}

			// Прекращаем после получения нескольких метрик
			if metricsReceived >= 5 {
				return
			}

		case <-timeout:
			if metricsReceived == 0 {
				t.Error("No metrics received from jobs channel")
			}
			return
		}
	}
}

func TestGetMetrics(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      3,
	}

	agent := New(cfg)
	agent.collectRuntimeMetrics()
	agent.collectGopsutilMetrics()

	metrics := agent.GetMetrics()

	if len(metrics) == 0 {
		t.Error("GetMetrics() should return metrics")
	}
	if _, exists := metrics["PollCount"]; !exists {
		t.Error("GetMetrics() should include PollCount")
	}
	if _, exists := metrics["Alloc"]; !exists {
		t.Error("GetMetrics() should include runtime metrics")
	}
	if _, exists := metrics["TotalMemory"]; !exists {
		t.Error("GetMetrics() should include TotalMemory")
	}

	// Проверяем, что возвращается копия, а не оригинал
	metrics["PollCount"].Counter = 999

	agent.mu.RLock()
	originalValue := agent.metrics["PollCount"].Counter
	agent.mu.RUnlock()

	if originalValue == 999 {
		t.Error("GetMetrics() should return copy, not original")
	}
}

func TestGetMetric(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      3,
	}

	agent := New(cfg)
	agent.collectRuntimeMetrics()

	// Тест существующей метрики
	metric, exists := agent.GetMetric("PollCount")
	if !exists {
		t.Error("GetMetric() should find existing metric")
	}
	if metric.Type != "counter" {
		t.Errorf("GetMetric() type = %v, want counter", metric.Type)
	}

	// Тест несуществующей метрики
	_, exists = agent.GetMetric("NonExistent")
	if exists {
		t.Error("GetMetric() should return false for non-existent metric")
	}
}

func TestThreadSafety(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      3,
	}

	agent := New(cfg)

	// Запускаем несколько горутин для одновременного доступа
	done := make(chan bool)

	// Горутина 1: читает метрики
	go func() {
		for i := 0; i < 100; i++ {
			agent.GetMetrics()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Горутина 2: собирает runtime метрики
	go func() {
		for i := 0; i < 100; i++ {
			agent.collectRuntimeMetrics()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Горутина 3: собирает gopsutil метрики
	go func() {
		for i := 0; i < 100; i++ {
			agent.collectGopsutilMetrics()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Ждем завершения всех горутин
	<-done
	<-done
	<-done

	// Если мы дошли сюда без race condition - тест пройден
}
