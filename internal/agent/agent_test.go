package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
)

func TestNewAgent(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
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
}

func TestCollectMetrics(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
	}

	agent := New(cfg)
	initialPollCount := agent.metrics["PollCount"].Counter

	agent.collectMetrics()

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

func TestBuildMetricURL(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
	}

	agent := New(cfg)

	tests := []struct {
		name     string
		metric   *MetricValue
		expected string
	}{
		{
			name:     "TestGauge",
			metric:   &MetricValue{Type: "gauge", Gauge: 123.45},
			expected: "http://localhost:8080/update/gauge/TestGauge/123.45",
		},
		{
			name:     "TestCounter",
			metric:   &MetricValue{Type: "counter", Counter: 42},
			expected: "http://localhost:8080/update/counter/TestCounter/42",
		},
		{
			name:     "InvalidType",
			metric:   &MetricValue{Type: "invalid"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := agent.buildMetricURL(tt.name, tt.metric)
			if url != tt.expected {
				t.Errorf("buildMetricURL() = %v, want %v", url, tt.expected)
			}
		})
	}
}

func TestSendMetric(t *testing.T) {
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

		var payload struct {
			ID    string   `json:"id"`
			MType string   `json:"type"`
			Delta *int64   `json:"delta,omitempty"`
			Value *float64 `json:"value,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Invalid JSON request: %v", err)
		}
		if payload.ID == "" || payload.MType == "" {
			t.Errorf("Invalid metric data in request body")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.AgentConfig{
		ServerAddress:  server.URL,
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
	}

	agent := New(cfg)

	if err := agent.sendMetric("TestGauge", &MetricValue{Type: "gauge", Gauge: 123.45}); err != nil {
		t.Errorf("sendMetric() error = %v", err)
	}
	if err := agent.sendMetric("TestCounter", &MetricValue{Type: "counter", Counter: 42}); err != nil {
		t.Errorf("sendMetric() error = %v", err)
	}
}

func TestSendMetricError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.AgentConfig{
		ServerAddress:  server.URL,
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
	}

	agent := New(cfg)

	err := agent.sendMetric("TestGauge", &MetricValue{Type: "gauge", Gauge: 123.45})
	if err == nil {
		t.Error("sendMetric() should return error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Error should mention status code 500, got: %v", err)
	}
}

func TestGetMetrics(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
	}

	agent := New(cfg)
	agent.collectMetrics()

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

	// Проверяем, что возвращается копия, а не оригинал
	metrics["PollCount"].Counter = 999
	if agent.metrics["PollCount"].Counter == 999 {
		t.Error("GetMetrics() should return copy, not original")
	}
}

func TestGetMetric(t *testing.T) {
	cfg := &config.AgentConfig{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
	}

	agent := New(cfg)
	agent.collectMetrics()

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
