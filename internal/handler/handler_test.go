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

// setupRouterJSON добавит JSON эндпоинты в тестовый роутер
func setupRouterJSON(handler *MetricHandler) chi.Router {
	r := chi.NewRouter()
	r.Post("/update", handler.UpdateMetricsJSON)
	r.Post("/value", handler.GetMetricJSON)
	return r
}

func TestUpdateMetricsJSON(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupRouterJSON(handler)

	tests := []struct {
		name       string
		payload    model.Metrics
		wantStatus int
		wantBody   string // частичная проверка
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		})
	}
}

func TestGetMetricJSON(t *testing.T) {
	repo := repository.New()
	svc := service.New(repo)
	handler := New(svc)
	router := setupRouterJSON(handler)

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
				MType: model.Counter, // неправильный тип
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
		})
	}
}

// Вспомогательные функции для указателей
func int64Ptr(i int64) *int64     { return &i }
func floatPtr(f float64) *float64 { return &f }
