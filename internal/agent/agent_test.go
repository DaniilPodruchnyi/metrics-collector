package agent

import (
	"context"
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

	// Проверяем инициализацию worker pool
	if agent.workerPool == nil {
		t.Error("Worker pool not initialized")
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

	// Используем контекст для вызова
	ctx := context.Background()
	if err := agent.sendSingleMetric(ctx, metric); err != nil {
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

	ctx := context.Background()
	err := agent.sendSingleMetric(ctx, metric)
	if err == nil {
		t.Error("sendSingleMetric() should return error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Error should mention status code 500, got: %v", err)
	}
}

func TestSendSingleMetricWithCancelledContext(t *testing.T) {
	// Сервер с задержкой
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
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

	value := 42.0
	metric := model.Metrics{
		ID:    "TestMetric",
		MType: "gauge",
		Value: &value,
	}

	// Создаем контекст с немедленной отменой
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Сразу отменяем

	err := agent.sendSingleMetric(ctx, metric)
	if err == nil {
		t.Error("sendSingleMetric() should return error for cancelled context")
	}
	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "cancel") {
		t.Errorf("Error should mention context cancellation, got: %v", err)
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

	// Запускаем worker pool для чтения из канала
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent.workerPool.Start(ctx)
	defer agent.workerPool.Stop()

	// Публикуем метрики
	agent.publishMetrics()

	// Даем время на обработку
	time.Sleep(100 * time.Millisecond)

	// Проверка прошла успешно, если не было паники
}

func TestMetricJobHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// Создаем тестовую метрику
	value := 42.0
	metric := model.Metrics{
		ID:    "TestMetric",
		MType: "gauge",
		Value: &value,
	}

	ctx := context.Background()
	err := agent.metricJobHandler(ctx, metric)
	if err != nil {
		t.Errorf("metricJobHandler() should not return error, got: %v", err)
	}

	// Тест с неправильным типом job
	err = agent.metricJobHandler(ctx, "invalid job type")
	if err == nil {
		t.Error("metricJobHandler() should return error for invalid job type")
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

func TestShutdown(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   100 * time.Millisecond,
		ReportInterval: 200 * time.Millisecond,
		RateLimit:      3,
	}

	agent := New(cfg)

	// Запускаем агента в горутине
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		agent.Run(ctx)
	}()

	// Даем агенту поработать
	time.Sleep(300 * time.Millisecond)

	// Отменяем контекст
	cancel()

	// Ждем завершения Run() с таймаутом
	// Run() сам вызовет Stop() для worker pool
	time.Sleep(2 * time.Second)

	// Проверяем, что worker pool остановлен
	if !agent.workerPool.IsStopped() {
		t.Error("Worker pool should be stopped after context cancellation")
	}
}

func TestShutdownExplicit(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   100 * time.Millisecond,
		ReportInterval: 200 * time.Millisecond,
		RateLimit:      3,
	}

	agent := New(cfg)

	// Запускаем агента в горутине
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		agent.Run(ctx)
	}()

	// Даем агенту поработать
	time.Sleep(300 * time.Millisecond)

	// Отменяем контекст
	cancel()

	// Явный вызов Shutdown с таймаутом
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err := agent.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Shutdown() should not return error, got: %v", err)
	}

	// Проверяем, что worker pool остановлен
	if !agent.workerPool.IsStopped() {
		t.Error("Worker pool should be stopped after explicit shutdown")
	}
}

func TestShutdownTimeout(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   100 * time.Millisecond,
		ReportInterval: 200 * time.Millisecond,
		RateLimit:      3,
	}

	agent := New(cfg)

	// Запускаем агента
	ctx, cancel := context.WithCancel(context.Background())
	go agent.Run(ctx)

	// Даем агенту поработать
	time.Sleep(300 * time.Millisecond)

	// Отменяем контекст
	cancel()

	// Пытаемся завершить с очень коротким таймаутом
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer shutdownCancel()

	err := agent.Shutdown(shutdownCtx)
	if err == nil {
		t.Error("Shutdown() should return timeout error for very short timeout")
	}

	// Даем время на нормальное завершение
	time.Sleep(2 * time.Second)
}

func TestShutdownIdempotent(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   100 * time.Millisecond,
		ReportInterval: 200 * time.Millisecond,
		RateLimit:      3,
	}

	agent := New(cfg)

	// Запускаем агента
	ctx, cancel := context.WithCancel(context.Background())
	go agent.Run(ctx)

	// Даем агенту поработать
	time.Sleep(300 * time.Millisecond)

	// Отменяем контекст
	cancel()

	shutdownCtx := context.Background()

	// Первый вызов Shutdown
	err1 := agent.Shutdown(shutdownCtx)
	if err1 != nil {
		t.Errorf("First Shutdown() should not return error, got: %v", err1)
	}

	// Второй вызов Shutdown (должен быть безопасным благодаря sync.Once)
	err2 := agent.Shutdown(shutdownCtx)
	if err2 != nil {
		t.Errorf("Second Shutdown() should not return error, got: %v", err2)
	}

	// Проверяем, что worker pool остановлен
	if !agent.workerPool.IsStopped() {
		t.Error("Worker pool should be stopped")
	}
}

func TestRuntimeCollector(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   50 * time.Millisecond,
		ReportInterval: 200 * time.Millisecond,
		RateLimit:      3,
	}

	agent := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	agent.wg.Add(1)
	go agent.runtimeCollector(ctx)

	// Ждем несколько циклов сбора
	time.Sleep(150 * time.Millisecond)

	// Проверяем, что метрики обновлялись
	agent.mu.RLock()
	pollCount := agent.metrics["PollCount"].Counter
	agent.mu.RUnlock()

	if pollCount < 2 {
		t.Errorf("Expected at least 2 poll cycles, got %d", pollCount)
	}

	// Ждем завершения
	agent.wg.Wait()
}

func TestGopsutilCollector(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   50 * time.Millisecond,
		ReportInterval: 200 * time.Millisecond,
		RateLimit:      3,
	}

	agent := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	agent.wg.Add(1)
	go agent.gopsutilCollector(ctx)

	// Ждем несколько циклов сбора
	time.Sleep(150 * time.Millisecond)

	// Проверяем, что метрики собраны
	agent.mu.RLock()
	totalMem := agent.metrics["TotalMemory"].Gauge
	agent.mu.RUnlock()

	if totalMem == 0 {
		t.Error("TotalMemory should be collected by gopsutil collector")
	}

	// Ждем завершения
	agent.wg.Wait()
}
