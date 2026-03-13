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
	ErrMetricNotFound      = errors.New("metric not found")
	ErrStorageFailure      = errors.New("storage operation failed")
)

// MetricsRepository определяет интерфейс для работы с хранилищем метрик
type MetricRepository interface {
	Store(metric *model.Metrics) error
	Get(name string) (*model.Metrics, bool, error)
	GetAll() (map[string]*model.Metrics, error)
	LoadData(data map[string]*model.Metrics) error
}

// BatchMetricRepository расширяет интерфейс для batch операций
type BatchMetricRepository interface {
	MetricRepository
	StoreBatch(metrics []model.Metrics) error
}

// MetricService инкапсулирует бизнес-логику работы с метриками.
type MetricService struct {
	repo MetricRepository
}

// New создает новый сервис для работы с метриками.
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

	existing, exists, err := s.repo.Get(name)
	if err != nil {
		return fmt.Errorf("%w: failed to get metric: %w", ErrStorageFailure, err)
	}

	if exists && existing.MType == model.Counter && existing.Delta != nil {
		// Увеличиваем существующее значение counter
		newDelta := *existing.Delta + delta
		existing.Delta = &newDelta
		if err := s.repo.Store(existing); err != nil {
			return fmt.Errorf("%w: failed to store metric: %w", ErrStorageFailure, err)
		}
	} else {
		// Создаем новую counter метрику
		metric := &model.Metrics{
			ID:    name,
			MType: model.Counter,
			Delta: &delta,
		}
		if err := s.repo.Store(metric); err != nil {
			return fmt.Errorf("%w: failed to store metric: %w", ErrStorageFailure, err)
		}
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

	existing, exists, err := s.repo.Get(name)
	if err != nil {
		return fmt.Errorf("%w: failed to get metric: %w", ErrStorageFailure, err)
	}

	if exists {
		// Обновляем существующую метрику
		existing.MType = model.Gauge
		existing.Value = &gaugeValue
		existing.Delta = nil
		if err := s.repo.Store(existing); err != nil {
			return fmt.Errorf("%w: failed to store metric: %w", ErrStorageFailure, err)
		}
	} else {
		// Создаем новую gauge метрику
		metric := &model.Metrics{
			ID:    name,
			MType: model.Gauge,
			Value: &gaugeValue,
		}
		if err := s.repo.Store(metric); err != nil {
			return fmt.Errorf("%w: failed to store metric: %w", ErrStorageFailure, err)
		}
	}

	return nil
}

// GetMetric возвращает метрику по имени
func (s *MetricService) GetMetric(name string) (*model.Metrics, bool) {
	metric, exists, err := s.repo.Get(name)
	if err != nil {
		return nil, false
	}
	return metric, exists
}

// GetAllMetrics возвращает все метрики
func (s *MetricService) GetAllMetrics() map[string]*model.Metrics {
	metrics, err := s.repo.GetAll()
	if err != nil {
		return make(map[string]*model.Metrics)
	}
	return metrics
}

// UpdateMetricsBatch обновляет множество метрик за один вызов
func (s *MetricService) UpdateMetricsBatch(metrics []model.Metrics) error {
	// Используем BatchStorer если repository его поддерживает
	if batchRepo, ok := s.repo.(BatchMetricRepository); ok {
		return batchRepo.StoreBatch(metrics)
	}

	// Fallback: обрабатываем по одной метрике
	for _, m := range metrics {
		var valueStr string
		if m.MType == model.Counter {
			if m.Delta == nil {
				return fmt.Errorf("missing delta for counter metric %s", m.ID)
			}
			valueStr = strconv.FormatInt(*m.Delta, 10)
		} else {
			if m.Value == nil {
				return fmt.Errorf("missing value for gauge metric %s", m.ID)
			}
			valueStr = strconv.FormatFloat(*m.Value, 'g', -1, 64)
		}

		if err := s.UpdateMetrics(m.MType, m.ID, valueStr); err != nil {
			return fmt.Errorf("failed to update metric %s: %w", m.ID, err)
		}
	}

	return nil
}
