package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/handler"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
	"github.com/go-chi/chi/v5"
)

// ExampleMetricHandler_UpdateMetricsJSON демонстрирует базовый запрос к эндпоинту POST /update.
func ExampleMetricHandler_UpdateMetricsJSON() {
	repo := repository.New()
	svc := service.New(repo)
	h := handler.New(svc, nil)

	r := chi.NewRouter()
	r.Post("/update", h.UpdateMetricsJSON)

	m := model.Metrics{
		ID:    "ExampleGauge",
		MType: model.Gauge,
	}
	value := 42.5
	m.Value = &value

	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(m)

	req := httptest.NewRequest(http.MethodPost, "/update", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	fmt.Println(w.Code)
	// Output:
	// 200
}

// ExampleMetricHandler_UpdateMetricsBatch демонстрирует работу с эндпоинтом POST /updates.
func ExampleMetricHandler_UpdateMetricsBatch() {
	repo := repository.New()
	svc := service.New(repo)
	h := handler.New(svc, nil)

	r := chi.NewRouter()
	r.Post("/updates", h.UpdateMetricsBatch)

	metrics := []model.Metrics{
		{ID: "BatchGauge", MType: model.Gauge, Value: floatPtr(1.23)},
		{ID: "BatchCounter", MType: model.Counter, Delta: int64Ptr(10)},
	}

	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(metrics)

	req := httptest.NewRequest(http.MethodPost, "/updates", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	fmt.Println(w.Code)
	// Output:
	// 200
}

func floatPtr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64     { return &v }
