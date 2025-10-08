package handler

import (
	"fmt"
	"net/http"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
)

// Структура содержащая handler для работы с метриками
type Handlers struct {
	metricService *service.MetricService
}

// Функция для инициализации handlers для работы с метриками
func New(metricService service.MetricService) *Handlers {
	return &Handlers{
		metricService: &metricService,
	}
}

// Handler для добавления/обновления метрик
func (h *Handlers) UpdateMetrics(w http.ResponseWriter, r *http.Request) {
	// Валидируем тип метода
	if r.Method != http.MethodPost {
		w.Write([]byte(fmt.Sprintf("Method %s not allowed", r.Method)))
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры из URL
	metricType := r.PathValue("type")
	metricName := r.PathValue("name")
	metricValue := r.PathValue("value")

	// Вызываем сервис по работе с метриками
	err := h.metricService.UpdateMetrics(metricType, metricName, metricValue)
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Запрос успешно завершился
	w.WriteHeader(http.StatusOK)
}
