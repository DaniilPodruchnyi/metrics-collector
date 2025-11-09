package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
	"github.com/go-chi/chi/v5"
)

// setupRouter создает роутер для всех эндпоинтов
func setupRouter(handler *MetricHandler) chi.Router {
	r := chi.NewRouter()

	// JSON endpoints
	r.Post("/update", handler.UpdateMetricsJSON)
	r.Post("/value", handler.GetMetricJSON)

	// Path-based endpoints
	r.Post("/update/{type}/{name}/{value}", handler.UpdateMetrics)
	r.Get("/value/{type}/{name}", handler.GetMetricValue)
	r.Get("/", handler.GetAllMetricsHTML)

	return r
}

// Вспомогательные функции для указателей
func int64Ptr(i int64) *int64     { return &i }
func floatPtr(f float64) *float64 { return &f }

// ============================================================================
// Тесты для UpdateMetrics (path-based)
// ============================================================================

func TestUpdateMetrics(t *testing.T) {
	tests := []struct {
		name       string
		metricType string
		metricName string
		metricVal  string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "valid counter",
			metricType: "counter",
			metricName: "TestCounter",
			metricVal:  "42",
			wantStatus: http.StatusOK,
			wantBody:   "OK",
		},
		{
			name:       "valid gauge",
			metricType: "gauge",
			metricName: "TestGauge",
			metricVal:  "123.456",
			wantStatus: http.StatusOK,
			wantBody:   "OK",
		},
		{
			name:       "invalid metric type",
			metricType: "unknown",
			metricName: "Test",
			metricVal:  "100",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid counter value",
			metricType: "counter",
			metricName: "Test",
			metricVal:  "not-a-number",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid gauge value",
			metricType: "gauge",
			metricName: "Test",
			metricVal:  "not-a-float",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.New()
			svc := service.New(repo)
			handler := New(svc)
			router := setupRouter(handler)

			url := "/update/" + tt.metricType + "/" + tt.metricName + "/" + tt.metricVal
			req := httptest.NewRequest(http.MethodPost, url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.wantBody, w.Body.String())
			}
		})
	}
}

// ============================================================================
// Тесты для GetMetricValue (path-based)
// ============================================================================

func TestGetMetricValue(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupRouter(handler)

	// Подготовка данных
	svc.UpdateMetrics("gauge", "TestGauge", "123.456")
	svc.UpdateMetrics("counter", "TestCounter", "42")

	tests := []struct {
		name       string
		metricType string
		metricName string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "existing gauge",
			metricType: "gauge",
			metricName: "TestGauge",
			wantStatus: http.StatusOK,
			wantBody:   "123.456",
		},
		{
			name:       "existing counter",
			metricType: "counter",
			metricName: "TestCounter",
			wantStatus: http.StatusOK,
			wantBody:   "42",
		},
		{
			name:       "non-existing metric",
			metricType: "gauge",
			metricName: "NoSuchMetric",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "type mismatch",
			metricType: "counter",
			metricName: "TestGauge",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/value/" + tt.metricType + "/" + tt.metricName
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.wantBody, w.Body.String())
			}
			if tt.wantStatus == http.StatusOK {
				contentType := w.Header().Get("Content-Type")
				if contentType != "text/plain" {
					t.Errorf("Expected Content-Type text/plain, got %s", contentType)
				}
			}
		})
	}
}

// ============================================================================
// Тесты для GetAllMetricsHTML
// ============================================================================

func TestGetAllMetricsHTML(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupRouter(handler)

	// Подготовка данных
	svc.UpdateMetrics("gauge", "Alloc", "1234.56")
	svc.UpdateMetrics("counter", "PollCount", "10")
	svc.UpdateMetrics("gauge", "RandomValue", "0.789")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html" {
		t.Errorf("Expected Content-Type text/html, got %s", contentType)
	}

	body := w.Body.String()

	// Проверяем наличие основных элементов HTML
	expectedStrings := []string{
		"<!DOCTYPE html>",
		"Metrics Dashboard",
		"<table>",
		"Alloc",
		"PollCount",
		"RandomValue",
		"Total metrics: 3",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected body to contain %q", expected)
		}
	}
}

func TestGetAllMetricsHTML_Empty(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
	if !strings.Contains(w.Body.String(), "Total metrics: 0") {
		t.Errorf("Expected empty metrics message")
	}
}

// ============================================================================
// Тесты для UpdateMetricsJSON
// ============================================================================

func TestUpdateMetricsJSON(t *testing.T) {
	tests := []struct {
		name       string
		payload    model.Metrics
		wantStatus int
		wantBody   string
	}{
		{
			name: "valid gauge",
			payload: model.Metrics{
				ID:    "TestGaugeJSON",
				MType: model.Gauge,
				Value: floatPtr(123.45),
			},
			wantStatus: http.StatusOK,
			wantBody:   `"id":"TestGaugeJSON"`,
		},
		{
			name: "valid counter",
			payload: model.Metrics{
				ID:    "TestCounterJSON",
				MType: model.Counter,
				Delta: int64Ptr(42),
			},
			wantStatus: http.StatusOK,
			wantBody:   `"id":"TestCounterJSON"`,
		},
		{
			name: "missing ID",
			payload: model.Metrics{
				MType: model.Gauge,
				Value: floatPtr(1.23),
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid metric data",
		},
		{
			name: "invalid type",
			payload: model.Metrics{
				ID:    "Test",
				MType: "unknown",
				Value: floatPtr(1.23),
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid metric data",
		},
		{
			name: "missing delta for counter",
			payload: model.Metrics{
				ID:    "BadCounter",
				MType: model.Counter,
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "Missing delta",
		},
		{
			name: "missing value for gauge",
			payload: model.Metrics{
				ID:    "BadGauge",
				MType: model.Gauge,
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "Missing value",
		},
		{
			name: "counter with zero delta",
			payload: model.Metrics{
				ID:    "ZeroCounter",
				MType: model.Counter,
				Delta: int64Ptr(0),
			},
			wantStatus: http.StatusOK,
			wantBody:   `"delta":0`,
		},
		{
			name: "gauge with negative value",
			payload: model.Metrics{
				ID:    "NegativeGauge",
				MType: model.Gauge,
				Value: floatPtr(-123.45),
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.New()
			svc := service.New(repo)
			handler := New(svc)
			router := setupRouter(handler)

			buf := new(bytes.Buffer)
			err := json.NewEncoder(buf).Encode(tt.payload)
			if err != nil {
				t.Fatalf("Failed to encode payload: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/update", buf)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.wantBody, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", contentType)
				}
			}
		})
	}
}

func TestUpdateMetricsJSON_InvalidJSON(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid JSON body") {
		t.Errorf("Expected error message about invalid JSON")
	}
}

// ============================================================================
// Тесты для GetMetricJSON
// ============================================================================

func TestGetMetricJSON(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupRouter(handler)

	// Подготовка метрик
	svc.UpdateMetrics("gauge", "TestGaugeJSON", "123.456")
	svc.UpdateMetrics("counter", "TestCounterJSON", "42")

	tests := []struct {
		name       string
		payload    model.Metrics
		wantStatus int
		wantBody   string
	}{
		{
			name: "existing gauge",
			payload: model.Metrics{
				ID:    "TestGaugeJSON",
				MType: model.Gauge,
			},
			wantStatus: http.StatusOK,
			wantBody:   `"value":123.456`,
		},
		{
			name: "existing counter",
			payload: model.Metrics{
				ID:    "TestCounterJSON",
				MType: model.Counter,
			},
			wantStatus: http.StatusOK,
			wantBody:   `"delta":42`,
		},
		{
			name: "missing ID",
			payload: model.Metrics{
				MType: model.Gauge,
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid metric data",
		},
		{
			name: "invalid type",
			payload: model.Metrics{
				ID:    "Test",
				MType: "unknown",
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid metric data",
		},
		{
			name: "non-existing metric",
			payload: model.Metrics{
				ID:    "NoSuchMetric",
				MType: model.Gauge,
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "Metric not found",
		},
		{
			name: "type mismatch",
			payload: model.Metrics{
				ID:    "TestGaugeJSON",
				MType: model.Counter,
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "type mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			err := json.NewEncoder(buf).Encode(tt.payload)
			if err != nil {
				t.Fatalf("Failed to encode payload: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/value", buf)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.wantBody, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", contentType)
				}
			}
		})
	}
}

func TestGetMetricJSON_InvalidJSON(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/value", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid JSON body") {
		t.Errorf("Expected error message about invalid JSON")
	}
}

// ============================================================================
// Интеграционные тесты
// ============================================================================

func TestCounterIncrement(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupRouter(handler)

	// Первое обновление
	payload1 := model.Metrics{
		ID:    "TestCounter",
		MType: model.Counter,
		Delta: int64Ptr(10),
	}
	buf1 := new(bytes.Buffer)
	json.NewEncoder(buf1).Encode(payload1)

	req1 := httptest.NewRequest(http.MethodPost, "/update", buf1)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("First update failed: %d", w1.Code)
	}

	// Второе обновление (должно прибавиться)
	payload2 := model.Metrics{
		ID:    "TestCounter",
		MType: model.Counter,
		Delta: int64Ptr(5),
	}
	buf2 := new(bytes.Buffer)
	json.NewEncoder(buf2).Encode(payload2)

	req2 := httptest.NewRequest(http.MethodPost, "/update", buf2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Second update failed: %d", w2.Code)
	}

	// Проверяем итоговое значение
	getPayload := model.Metrics{
		ID:    "TestCounter",
		MType: model.Counter,
	}
	buf3 := new(bytes.Buffer)
	json.NewEncoder(buf3).Encode(getPayload)

	req3 := httptest.NewRequest(http.MethodPost, "/value", buf3)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("Get value failed: %d", w3.Code)
	}
	if !strings.Contains(w3.Body.String(), `"delta":15`) {
		t.Errorf("Expected delta to be 15, got: %s", w3.Body.String())
	}
}

func TestGaugeOverwrite(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupRouter(handler)

	// Первое обновление
	payload1 := model.Metrics{
		ID:    "TestGauge",
		MType: model.Gauge,
		Value: floatPtr(100.5),
	}
	buf1 := new(bytes.Buffer)
	json.NewEncoder(buf1).Encode(payload1)

	req1 := httptest.NewRequest(http.MethodPost, "/update", buf1)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("First update failed: %d", w1.Code)
	}

	// Второе обновление (должно перезаписаться)
	payload2 := model.Metrics{
		ID:    "TestGauge",
		MType: model.Gauge,
		Value: floatPtr(200.7),
	}
	buf2 := new(bytes.Buffer)
	json.NewEncoder(buf2).Encode(payload2)

	req2 := httptest.NewRequest(http.MethodPost, "/update", buf2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Second update failed: %d", w2.Code)
	}

	// Проверяем итоговое значение
	getPayload := model.Metrics{
		ID:    "TestGauge",
		MType: model.Gauge,
	}
	buf3 := new(bytes.Buffer)
	json.NewEncoder(buf3).Encode(getPayload)

	req3 := httptest.NewRequest(http.MethodPost, "/value", buf3)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("Get value failed: %d", w3.Code)
	}
	if !strings.Contains(w3.Body.String(), `"value":200.7`) {
		t.Errorf("Expected value to be 200.7, got: %s", w3.Body.String())
	}
}
