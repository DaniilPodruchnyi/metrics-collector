package service

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
)

// Предопределенные ошибки для разных случаев
var (
	ErrInvalidMetricType   = errors.New("invalid metric type")
	ErrInvalidCounterValue = errors.New("invalid counter value format")
	ErrInvalidGaugeValue   = errors.New("invalid gauge value format")
)

// MetricsRepository определяет интерфейс для работы с хранилищем метрик
type MetricRepository interface {
	Store(metric *model.Metrics)
	Get(name string) (*model.Metrics, bool)
	GetAll() map[string]*model.Metrics
	LoadData(data map[string]*model.Metrics)
}

// Структура сервиса по работе с метриками
type MetricService struct {
	repo MetricRepository
}

// Функция для инициализации сервиса по работе с метриками
func New(repo MetricRepository) *MetricService {
	return &MetricService{
		repo: repo,
	}
}

// UpdateMetrics обновляет/добавляет метрику
func (s *MetricService) UpdateMetrics(metricType, metricName, metricValue string) error {
	// Валидация типа метрики
	if metricType != model.Counter && metricType != model.Gauge {
		return fmt.Errorf("%w: got %s, expected %s or %s",
			ErrInvalidMetricType, metricType, model.Counter, model.Gauge)
	}

	// Обрабатываем counter
	if metricType == model.Counter {
		return s.updateCounter(metricName, metricValue)
	}

	// Обрабатываем gauge
	return s.updateGauge(metricName, metricValue)
}

// updateCounter обрабатывает метрику типа counter
func (s *MetricService) updateCounter(name, value string) error {
	delta, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: cannot parse '%s' as int64: %w",
			ErrInvalidCounterValue, value, err)
	}

	existing, exists := s.repo.Get(name)

	if exists && existing.MType == model.Counter && existing.Delta != nil {
		// Увеличиваем существующее значение counter
		newDelta := *existing.Delta + delta
		existing.Delta = &newDelta
		s.repo.Store(existing)
	} else {
		// Создаем новую counter метрику
		metric := &model.Metrics{
			ID:    name,
			MType: model.Counter,
			Delta: &delta,
		}
		s.repo.Store(metric)
	}

	return nil
}

// updateGauge обрабатывает метрику типа gauge
func (s *MetricService) updateGauge(name, value string) error {
	gaugeValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("%w: cannot parse '%s' as float64: %w",
			ErrInvalidGaugeValue, value, err)
	}

	existing, exists := s.repo.Get(name)

	if exists {
		// Обновляем существующую метрику
		existing.MType = model.Gauge
		existing.Value = &gaugeValue
		existing.Delta = nil // Очищаем Delta для gauge
		s.repo.Store(existing)
	} else {
		// Создаем новую gauge метрику
		metric := &model.Metrics{
			ID:    name,
			MType: model.Gauge,
			Value: &gaugeValue,
		}
		s.repo.Store(metric)
	}

	return nil
}

// GetMetric возвращает метрику по имени
func (s *MetricService) GetMetric(name string) (*model.Metrics, bool) {
	return s.repo.Get(name)
}

// GetAllMetrics возвращает все метрики
func (s *MetricService) GetAllMetrics() map[string]*model.Metrics {
	return s.repo.GetAll()
}
