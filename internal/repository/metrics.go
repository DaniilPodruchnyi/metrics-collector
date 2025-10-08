package repository

import (
	"fmt"
	"strconv"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
)

// Структура для работы с хранилищем данных метрик
type MemStorage struct {
	data map[string]*model.Metrics
}

// Функция для инициализации хранилища
func New() *MemStorage {
	return &MemStorage{
		data: make(map[string]*model.Metrics),
	}
}

// Метод для обновления/добавления метрики в хранилище
func (r *MemStorage) UpdateMetrics(metricType, metricName, metricValue string) error {
	// Обрабатываем counter
	if metricType == model.Counter {
		return r.updateCounter(metricName, metricValue)
	}

	// Обрабатываем gauge
	return r.updateGauge(metricName, metricValue)
}

// Метод для обработки метрики типа counter
func (r *MemStorage) updateCounter(name, value string) error {
	delta, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("incorrect counter value format: %w", err)
	}

	existing, exists := r.data[name]

	if exists {
		// Если метрика существует и это counter - увеличиваем значение
		if existing.MType == model.Counter {
			*existing.Delta += delta
		} else {
			// Если метрика существовала как gauge - меняем тип и устанавливаем значение
			existing.MType = model.Counter
			if existing.Delta == nil {
				existing.Delta = &delta
			} else {
				*existing.Delta = delta
			}
		}
	} else {
		// Создаем новую counter метрику
		r.data[name] = &model.Metrics{
			ID:    name,
			MType: model.Counter,
			Delta: &delta,
		}
	}

	// Выводим все метрики
	// for k, v := range r.data {
	// 	if v.Delta != nil {
	// 		fmt.Printf("k: %s | vDELTA: %d\n", k, *v.Delta)
	// 	}
	// 	if v.Value != nil {
	// 		fmt.Printf("k: %s | vValue: %f\n", k, *v.Value)
	// 	}
	// }

	return nil
}

// Метод для обработки метрики типа gauge
func (r *MemStorage) updateGauge(name, value string) error {
	gaugeValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("incorrect gauge value format: %w", err)
	}

	existing, exists := r.data[name]

	if exists {
		// Обновляем существующую метрику - меняем тип и значение
		existing.MType = model.Gauge
		if existing.Value == nil {
			existing.Value = &gaugeValue
		} else {
			*existing.Value = gaugeValue
		}
	} else {
		// Создаем новую gauge метрику
		r.data[name] = &model.Metrics{
			ID:    name,
			MType: model.Gauge,
			Value: &gaugeValue,
		}
	}

	// Выводим все метрики
	// for k, v := range r.data {
	// 	if v.Delta != nil {
	// 		fmt.Printf("k: %s | vDELTA: %d\n", k, *v.Delta)
	// 	}
	// 	if v.Value != nil {
	// 		fmt.Printf("k: %s | vValue: %f\n", k, *v.Value)
	// 	}
	// }

	return nil
}
