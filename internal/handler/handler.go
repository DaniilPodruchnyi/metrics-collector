package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
)

type MetricHandler struct {
	service *service.MetricService
}

// Функция для инициализации handlers
func New(svc *service.MetricService) *MetricHandler {
	return &MetricHandler{
		service: svc,
	}
}

// getHTTPStatusFromError определяет HTTP статус код на основе ошибки
func getHTTPStatusFromError(err error) int {
	switch {
	case errors.Is(err, service.ErrInvalidMetricType):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrInvalidCounterValue):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrInvalidGaugeValue):
		return http.StatusBadRequest
	case strings.Contains(err.Error(), "validation"):
		return http.StatusBadRequest
	case strings.Contains(err.Error(), "not found"):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// UpdateMetrics обрабатывает обновление метрик
func (h *MetricHandler) UpdateMetrics(w http.ResponseWriter, r *http.Request) {
	// Извлекаем параметры из пути с помощью Go 1.22 PathValue
	metricType := r.PathValue("type")
	metricName := r.PathValue("name")
	metricValue := r.PathValue("value")

	// Базовая валидация параметров (на случай если PathValue вернет пустые строки)
	if metricType == "" {
		http.Error(w, "metric type is required", http.StatusBadRequest)
		return
	}
	if metricName == "" {
		http.Error(w, "metric name is required", http.StatusBadRequest)
		return
	}
	if metricValue == "" {
		http.Error(w, "metric value is required", http.StatusBadRequest)
		return
	}

	err := h.service.UpdateMetrics(metricType, metricName, metricValue)
	if err != nil {
		statusCode := getHTTPStatusFromError(err)
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// GetMetric возвращает метрику по имени
func (h *MetricHandler) GetMetric(w http.ResponseWriter, r *http.Request) {
	// Извлекаем имя метрики из пути
	name := r.PathValue("name")

	if name == "" {
		http.Error(w, "metric name is required", http.StatusBadRequest)
		return
	}

	metric, exists := h.service.GetMetric(name)
	if !exists {
		http.Error(w, fmt.Sprintf("metric '%s' not found", name), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metric)
}

// GetAllMetrics возвращает все метрики
func (h *MetricHandler) GetAllMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.service.GetAllMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}
