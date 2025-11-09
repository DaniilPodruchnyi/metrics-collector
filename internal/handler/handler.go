package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
	"github.com/go-chi/chi/v5"
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
	// Извлекаем параметры из пути с помощью chi
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")
	metricValue := chi.URLParam(r, "value")

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
	name := chi.URLParam(r, "name")

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

// GetMetricValue возвращает значение метрики в текстовом виде
func (h *MetricHandler) GetMetricValue(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")

	if metricType == "" || metricName == "" {
		http.Error(w, "metric type and name are required", http.StatusBadRequest)
		return
	}

	metric, exists := h.service.GetMetric(metricName)
	if !exists {
		http.Error(w, fmt.Sprintf("metric '%s' not found", metricName), http.StatusNotFound)
		return
	}

	// Проверяем соответствие типа
	if metric.MType != metricType {
		http.Error(w, fmt.Sprintf("metric '%s' is not of type '%s'", metricName, metricType), http.StatusNotFound)
		return
	}

	// Возвращаем значение в текстовом виде
	var value string
	switch metricType {
	case "counter":
		if metric.Delta != nil {
			value = strconv.FormatInt(*metric.Delta, 10)
		} else {
			value = "0"
		}
	case "gauge":
		if metric.Value != nil {
			value = strconv.FormatFloat(*metric.Value, 'f', -1, 64)
		} else {
			value = "0"
		}
	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(value))
}

// GetAllMetricsHTML возвращает HTML-страницу со всеми метриками
func (h *MetricHandler) GetAllMetricsHTML(w http.ResponseWriter, r *http.Request) {
	metrics := h.service.GetAllMetrics()

	// HTML-шаблон для отображения метрик
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Metrics</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background-color: #f2f2f2; }
        .counter { color: #007bff; }
        .gauge { color: #28a745; }
    </style>
</head>
<body>
    <h1>Metrics Dashboard</h1>
    <table>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Value</th>
        </tr>
        {{range $name, $metric := .}}
        <tr>
            <td>{{$name}}</td>
            <td class="{{$metric.MType}}">{{$metric.MType}}</td>
            <td>
                {{if eq $metric.MType "counter"}}
                    {{if $metric.Delta}}{{.Delta}}{{else}}0{{end}}
                {{else if eq $metric.MType "gauge"}}
                    {{if $metric.Value}}{{.Value}}{{else}}0{{end}}
                {{end}}
            </td>
        </tr>
        {{end}}
    </table>
    <p>Total metrics: {{len .}}</p>
</body>
</html>`

	t, err := template.New("metrics").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	err = t.Execute(w, metrics)
	if err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
		return
	}
}

// UpdateMetricsJSON принимает JSON с метрикой в теле POST /update
func (h *MetricHandler) UpdateMetricsJSON(w http.ResponseWriter, r *http.Request) {
	var m model.Metrics
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "Invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if m.ID == "" || (m.MType != model.Gauge && m.MType != model.Counter) {
		http.Error(w, "Invalid metric data: missing ID or invalid type", http.StatusBadRequest)
		return
	}

	var metricVal string
	if m.MType == model.Counter {
		if m.Delta == nil {
			http.Error(w, "Missing delta for counter metric", http.StatusBadRequest)
			return
		}
		metricVal = strconv.FormatInt(*m.Delta, 10)
	} else { // gauge
		if m.Value == nil {
			http.Error(w, "Missing value for gauge metric", http.StatusBadRequest)
			return
		}
		metricVal = strconv.FormatFloat(*m.Value, 'g', -1, 64)
	}

	if err := h.service.UpdateMetrics(m.MType, m.ID, metricVal); err != nil {
		status := getHTTPStatusFromError(err)
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Можно вернуть подтверждение в JSON, например:
	json.NewEncoder(w).Encode(m)
}

// GetMetricJSON - POST /value принимает ID и MType в JSON и возвращает метрику с заполненными значениями
func (h *MetricHandler) GetMetricJSON(w http.ResponseWriter, r *http.Request) {
	var req model.Metrics
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.ID == "" || (req.MType != model.Gauge && req.MType != model.Counter) {
		http.Error(w, "Invalid metric data: missing ID or invalid type", http.StatusBadRequest)
		return
	}

	metric, exists := h.service.GetMetric(req.ID)
	if !exists || metric.MType != req.MType {
		http.Error(w, "Metric not found or type mismatch", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metric)
}
