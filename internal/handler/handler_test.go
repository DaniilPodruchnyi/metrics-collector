package handler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
	"github.com/go-chi/chi/v5"
)

func setupTestRouter(handler *MetricHandler) chi.Router {
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", handler.UpdateMetrics)
	r.Get("/value/{type}/{name}", handler.GetMetricValue)
	r.Get("/", handler.GetAllMetricsHTML)
	return r
}

func TestUpdateMetrics(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupTestRouter(handler)

	tests := []struct {
		name         string
		method       string
		url          string
		wantStatus   int
		wantBody     string
		checkMetric  bool
		metricName   string
		expectedType string
		expectedVal  string
	}{
		{
			name:         "valid gauge",
			method:       http.MethodPost,
			url:          "/update/gauge/TestGauge/123.45",
			wantStatus:   http.StatusOK,
			checkMetric:  true,
			metricName:   "TestGauge",
			expectedType: "gauge",
			expectedVal:  "123.45",
		},
		{
			name:         "valid counter",
			method:       http.MethodPost,
			url:          "/update/counter/TestCounter/42",
			wantStatus:   http.StatusOK,
			checkMetric:  true,
			metricName:   "TestCounter",
			expectedType: "counter",
			expectedVal:  "42",
		},
		{
			name:       "invalid HTTP method",
			method:     http.MethodGet,
			url:        "/update/gauge/Test/1",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "invalid metric type",
			method:     http.MethodPost,
			url:        "/update/invalid/Test/1",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid metric type",
		},
		{
			name:       "invalid counter value",
			method:     http.MethodPost,
			url:        "/update/counter/BadCounter/not_a_number",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid counter value format",
		},
		{
			name:       "invalid gauge value",
			method:     http.MethodPost,
			url:        "/update/gauge/BadGauge/not_a_float",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid gauge value format",
		},
		{
			name:         "counter increment test",
			method:       http.MethodPost,
			url:          "/update/counter/IncrementTest/10",
			wantStatus:   http.StatusOK,
			checkMetric:  true,
			metricName:   "IncrementTest",
			expectedType: "counter",
			expectedVal:  "10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantBody != "" {
				body := strings.TrimSpace(w.Body.String())
				if !strings.Contains(body, tt.wantBody) {
					t.Errorf("Expected error body to contain %q, got %q", tt.wantBody, body)
				}
			}

			// Проверяем через сервис
			if tt.checkMetric {
				metric, exists := svc.GetMetric(tt.metricName)
				if !exists {
					t.Fatalf("Metric %s was not saved", tt.metricName)
				}

				if metric.MType != tt.expectedType {
					t.Errorf("Metric type mismatch: got %s, want %s", metric.MType, tt.expectedType)
				}

				switch tt.expectedType {
				case "counter":
					if metric.Delta == nil {
						t.Fatal("Delta is nil for counter")
					}
					expected, _ := strconv.ParseInt(tt.expectedVal, 10, 64)
					if *metric.Delta != expected {
						t.Errorf("Counter value mismatch: got %d, want %d", *metric.Delta, expected)
					}
				case "gauge":
					if metric.Value == nil {
						t.Fatal("Value is nil for gauge")
					}
					expected, _ := strconv.ParseFloat(tt.expectedVal, 64)
					if *metric.Value != expected {
						t.Errorf("Gauge value mismatch: got %f, want %f", *metric.Value, expected)
					}
				}
			}
		})
	}
}

func TestSpecialFloatValues(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupTestRouter(handler)

	tests := []struct {
		name       string
		url        string
		wantStatus int
		shouldWork bool
	}{
		{
			name:       "positive infinity",
			url:        "/update/gauge/TestPosInf/+inf",
			wantStatus: http.StatusOK,
			shouldWork: true,
		},
		{
			name:       "negative infinity",
			url:        "/update/gauge/TestNegInf/-inf",
			wantStatus: http.StatusOK,
			shouldWork: true,
		},
		{
			name:       "mixed letters and numbers",
			url:        "/update/gauge/TestInvalid/123abc",
			wantStatus: http.StatusBadRequest,
			shouldWork: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestCounterIncrement(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupTestRouter(handler)

	// Первое обновление counter
	req1 := httptest.NewRequest(http.MethodPost, "/update/counter/TestIncrement/5", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("First update failed with status %d", w1.Code)
	}

	// Второе обновление того же counter
	req2 := httptest.NewRequest(http.MethodPost, "/update/counter/TestIncrement/3", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Second update failed with status %d", w2.Code)
	}

	// Проверяем, что значение увеличилось
	metric, exists := svc.GetMetric("TestIncrement")
	if !exists {
		t.Fatal("Metric was not found")
	}

	if metric.Delta == nil {
		t.Fatal("Delta is nil")
	}

	expected := int64(8) // 5 + 3
	if *metric.Delta != expected {
		t.Errorf("Expected counter value %d, got %d", expected, *metric.Delta)
	}
}

// Тест для GetMetricValue - возвращает текстовое значение
func TestGetMetricValue(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupTestRouter(handler)

	// Добавляем тестовые метрики
	svc.UpdateMetrics("gauge", "TestGauge", "123.45")
	svc.UpdateMetrics("counter", "TestCounter", "42")

	tests := []struct {
		name       string
		url        string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "existing gauge",
			url:        "/value/gauge/TestGauge",
			wantStatus: http.StatusOK,
			wantBody:   "123.45",
		},
		{
			name:       "existing counter",
			url:        "/value/counter/TestCounter",
			wantStatus: http.StatusOK,
			wantBody:   "42",
		},
		{
			name:       "non-existing metric",
			url:        "/value/gauge/NonExisting",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "wrong type for existing metric",
			url:        "/value/counter/TestGauge", // TestGauge is gauge, not counter
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			// Проверяем Content-Type для успешных запросов
			if w.Code == http.StatusOK {
				contentType := w.Header().Get("Content-Type")
				if contentType != "text/plain" {
					t.Errorf("Expected Content-Type text/plain, got %s", contentType)
				}

				if tt.wantBody != "" {
					body := strings.TrimSpace(w.Body.String())
					if body != tt.wantBody {
						t.Errorf("Expected body %q, got %q", tt.wantBody, body)
					}
				}
			}
		})
	}
}

// Тест для HTML страницы со всеми метриками
func TestGetAllMetricsHTML(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupTestRouter(handler)

	// Добавляем тестовые метрики
	svc.UpdateMetrics("gauge", "TestGauge", "123.45")
	svc.UpdateMetrics("counter", "TestCounter", "42")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Проверяем Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html" {
		t.Errorf("Expected Content-Type text/html, got %s", contentType)
	}

	body := w.Body.String()

	// Проверяем наличие HTML-элементов
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("Response should contain HTML doctype")
	}

	if !strings.Contains(body, "<title>Metrics</title>") {
		t.Error("Response should contain title")
	}

	if !strings.Contains(body, "TestGauge") {
		t.Error("Response should contain TestGauge metric")
	}

	if !strings.Contains(body, "TestCounter") {
		t.Error("Response should contain TestCounter metric")
	}

	if !strings.Contains(body, "123.45") {
		t.Error("Response should contain gauge value")
	}

	if !strings.Contains(body, "42") {
		t.Error("Response should contain counter value")
	}

	// Проверяем наличие таблицы
	if !strings.Contains(body, "<table>") {
		t.Error("Response should contain table")
	}
}

func TestTypeConversion(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupTestRouter(handler)

	// Создаем метрику как gauge
	req1 := httptest.NewRequest(http.MethodPost, "/update/gauge/TestConversion/100.5", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	// Преобразуем в counter
	req2 := httptest.NewRequest(http.MethodPost, "/update/counter/TestConversion/50", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Type conversion failed with status %d", w2.Code)
	}

	// Проверяем, что тип изменился на counter
	metric, exists := svc.GetMetric("TestConversion")
	if !exists {
		t.Fatal("Metric not found")
	}

	if metric.MType != "counter" {
		t.Errorf("Expected type counter, got %s", metric.MType)
	}

	if metric.Delta == nil || *metric.Delta != 50 {
		t.Errorf("Expected counter value 50, got %v", metric.Delta)
	}
}

// Тест для проверки текстового вывода значений после обновления
func TestMetricValueAfterUpdate(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupTestRouter(handler)

	// Обновляем counter несколько раз
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/update/counter/TestSum/10", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/update/counter/TestSum/5", nil))

	// Получаем значение в текстовом виде
	req := httptest.NewRequest("GET", "/value/counter/TestSum", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	body := strings.TrimSpace(w.Body.String())
	if body != "15" { // 10 + 5
		t.Errorf("Expected counter value '15', got '%s'", body)
	}

	// Проверяем gauge
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/update/gauge/TestFloat/3.14159", nil))

	req2 := httptest.NewRequest("GET", "/value/gauge/TestFloat", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w2.Code)
	}

	body2 := strings.TrimSpace(w2.Body.String())
	if body2 != "3.14159" {
		t.Errorf("Expected gauge value '3.14159', got '%s'", body2)
	}
}
