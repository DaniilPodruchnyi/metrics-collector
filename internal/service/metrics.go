package service

import (
	"fmt"
	"strconv"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
)

// Структура сервиса по работе с метриками
type MetricService struct {
	metricRepository *repository.MemStorage
}

// Функция для инициализации сервиса по работе с метриками
func New(metricRepository repository.MemStorage) *MetricService {
	return &MetricService{
		metricRepository: &metricRepository,
	}
}

// Метод для обновления/добавления метрик
func (s *MetricService) UpdateMetrics(metricType, metricName, metricValue string) error {
	// Проверка типа метрики
	if metricType != model.Counter && metricType != model.Gauge {
		return fmt.Errorf("incorrect type of metric")
	}

	// Проверка значения Counter (тип int64)
	_, err := strconv.ParseInt(metricValue, 10, 64)
	if err != nil && metricType == model.Counter {
		return fmt.Errorf("incorrect type of counter value")
	}

	// Проверка значения Gauge (тип floag64)
	_, err = strconv.ParseFloat(metricValue, 64)
	if err != nil && metricType == model.Gauge {
		return fmt.Errorf("incorrect type of gauge value")
	}

	// Передаем провалидированные данные для сохранения в хранилище
	err = s.metricRepository.UpdateMetrics(metricType, metricName, metricValue)
	if err != nil {
		return err
	}

	// В случае успеха - ошибок нет
	return nil
}
