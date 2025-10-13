package repository

import (
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

// Store сохраняет метрику в хранилище
func (r *MemStorage) Store(metric *model.Metrics) {
	r.data[metric.ID] = metric
}

// Get возвращает метрику по имени
func (r *MemStorage) Get(name string) (*model.Metrics, bool) {
	metric, ok := r.data[name]
	return metric, ok
}

// GetAll возвращает все метрики
func (r *MemStorage) GetAll() map[string]*model.Metrics {
	result := make(map[string]*model.Metrics)
	for k, v := range r.data {
		result[k] = v
	}
	return result
}
