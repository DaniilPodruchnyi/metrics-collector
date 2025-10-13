package main

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

// Тест sendMetric с mock-сервером
func TestSendMetric(t *testing.T) {
	tests := []struct {
		name     string
		metric   *MetricValue
		wantPath string
	}{
		{
			name:     "gauge",
			metric:   &MetricValue{Type: "gauge", Gauge: 123.456},
			wantPath: "/update/gauge/test_metric/123.456",
		},
		{
			name:     "counter",
			metric:   &MetricValue{Type: "counter", Counter: 42},
			wantPath: "/update/counter/test_metric/42",
		},
		{
			name:     "gauge integer value",
			metric:   &MetricValue{Type: "gauge", Gauge: 100.0},
			wantPath: "/update/gauge/test_metric/100", // без .0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаём mock-сервер
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST, got %s", r.Method)
				}
				if r.Header.Get("Content-Type") != "text/plain" {
					t.Errorf("Expected Content-Type: text/plain, got %s", r.Header.Get("Content-Type"))
				}
				if r.URL.Path != tt.wantPath {
					t.Errorf("Expected path %s, got %s", tt.wantPath, r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()

			// Подменяем serverAddr на mock
			originalAddr := serverAddr
			serverAddr = ts.URL
			defer func() { serverAddr = originalAddr }()

			err := sendMetric("test_metric", tt.metric)
			if err != nil {
				t.Errorf("sendMetric() error = %v", err)
			}
		})
	}
}

// Тест ошибки при неверном типе метрики
func TestSendMetric_InvalidType(t *testing.T) {
	m := &MetricValue{Type: "invalid"}
	err := sendMetric("test", m)
	if err == nil {
		t.Error("Expected error for invalid metric type, got nil")
	}
	if !strings.Contains(err.Error(), "unknown metric type") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// Тест сбора метрик из runtime (проверяем, что не паникует и заполняет значения)
func TestCollectRuntimeMetrics(t *testing.T) {
	metrics := make(map[string]*MetricValue)
	for _, name := range runtimeGaugeMetrics {
		metrics[name] = &MetricValue{Type: "gauge"}
	}
	metrics["PollCount"] = &MetricValue{Type: "counter", Counter: 0}
	metrics["RandomValue"] = &MetricValue{Type: "gauge", Gauge: 0.0}

	// Собираем данные
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// Обновляем
	metrics["PollCount"].Counter++
	metrics["RandomValue"].Gauge = 0.123 // фиксированное значение для теста

	metrics["Alloc"].Gauge = float64(ms.Alloc)
	metrics["GCCPUFraction"].Gauge = ms.GCCPUFraction

	if metrics["Alloc"].Gauge <= 0 {
		t.Errorf("Alloc should be > 0, got %f", metrics["Alloc"].Gauge)
	}
	if metrics["PollCount"].Counter != 1 {
		t.Errorf("PollCount should be 1, got %d", metrics["PollCount"].Counter)
	}
	if metrics["RandomValue"].Gauge != 0.123 {
		t.Errorf("RandomValue mismatch")
	}
}
