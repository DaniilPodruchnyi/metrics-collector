package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
)

func TestUpdateMetrics(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)

	// Настраиваем ServeMux
	mux := http.NewServeMux()
	mux.HandleFunc("POST /update/{type}/{name}/{value}", handler.UpdateMetrics)

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

			mux.ServeHTTP(w, req)

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

// Ттест для проверки специальных float значений
func TestSpecialFloatValues(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /update/{type}/{name}/{value}", handler.UpdateMetrics)

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

			mux.ServeHTTP(w, req)

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

	mux := http.NewServeMux()
	mux.HandleFunc("POST /update/{type}/{name}/{value}", handler.UpdateMetrics)

	// Первое обновление counter
	req1 := httptest.NewRequest(http.MethodPost, "/update/counter/TestIncrement/5", nil)
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("First update failed with status %d", w1.Code)
	}

	// Второе обновление того же counter
	req2 := httptest.NewRequest(http.MethodPost, "/update/counter/TestIncrement/3", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

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

func TestGetMetric(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)

	// Добавляем тестовые метрики
	svc.UpdateMetrics("gauge", "TestGauge", "123.45")
	svc.UpdateMetrics("counter", "TestCounter", "42")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /value/{name}", handler.GetMetric)

	tests := []struct {
		name       string
		url        string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "existing gauge",
			url:        "/value/TestGauge",
			wantStatus: http.StatusOK,
		},
		{
			name:       "existing counter",
			url:        "/value/TestCounter",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existing metric",
			url:        "/value/NonExisting",
			wantStatus: http.StatusNotFound,
			wantBody:   "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantBody != "" {
				body := strings.TrimSpace(w.Body.String())
				if !strings.Contains(body, tt.wantBody) {
					t.Errorf("Expected body to contain %q, got %q", tt.wantBody, body)
				}
			}

			// Для успешных запросов проверяем JSON
			if w.Code == http.StatusOK {
				var metric model.Metrics
				err := json.NewDecoder(w.Body).Decode(&metric)
				if err != nil {
					t.Errorf("Failed to decode JSON response: %v", err)
				}

				if metric.ID == "" {
					t.Error("Metric ID is empty")
				}
			}
		})
	}
}

func TestGetAllMetrics(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)

	// Добавляем тестовые метрики
	svc.UpdateMetrics("gauge", "TestGauge", "123.45")
	svc.UpdateMetrics("counter", "TestCounter", "42")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handler.GetAllMetrics)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Проверяем Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Проверяем, что возвращается валидный JSON
	var metrics map[string]*model.Metrics
	err := json.NewDecoder(w.Body).Decode(&metrics)
	if err != nil {
		t.Errorf("Failed to decode JSON response: %v", err)
	}

	// Проверяем, что метрики присутствуют
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(metrics))
	}

	if _, exists := metrics["TestGauge"]; !exists {
		t.Error("TestGauge metric not found in response")
	}

	if _, exists := metrics["TestCounter"]; !exists {
		t.Error("TestCounter metric not found in response")
	}
}

func TestTypeConversion(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /update/{type}/{name}/{value}", handler.UpdateMetrics)

	// Создаем метрику как gauge
	req1 := httptest.NewRequest(http.MethodPost, "/update/gauge/TestConversion/100.5", nil)
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)

	// Преобразуем в counter
	req2 := httptest.NewRequest(http.MethodPost, "/update/counter/TestConversion/50", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

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
