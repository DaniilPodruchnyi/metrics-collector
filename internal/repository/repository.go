package repository

import (
	"sync"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
)

// MetricsRepository определяет интерфейс для работы с хранилищем метрик
type MetricRepository interface {
	Store(metric *model.Metrics) error
	Get(name string) (*model.Metrics, bool, error)
	GetAll() (map[string]*model.Metrics, error)
	LoadData(data map[string]*model.Metrics) error
}

// Структура для работы с хранилищем данных метрик
type MemStorage struct {
	data map[string]*model.Metrics
	mu   sync.RWMutex
}

// Функция для инициализации хранилища
func New() *MemStorage {
	return &MemStorage{
		data: make(map[string]*model.Metrics),
	}
}

// NewWithData создает хранилище с предзагруженными данными
func NewWithData(data map[string]*model.Metrics) *MemStorage {
	return &MemStorage{
		data: data,
	}
}

// Store сохраняет метрику в хранилище
func (r *MemStorage) Store(metric *model.Metrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[metric.ID] = metric
	return nil
}

// Get возвращает метрику по имени
func (r *MemStorage) Get(name string) (*model.Metrics, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	metric, ok := r.data[name]
	return metric, ok, nil
}

// GetAll возвращает все метрики
func (r *MemStorage) GetAll() (map[string]*model.Metrics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*model.Metrics, len(r.data))
	for k, v := range r.data {
		result[k] = v
	}
	return result, nil
}

// LoadData загружает данные в хранилище (для восстановления из файла)
func (r *MemStorage) LoadData(data map[string]*model.Metrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = data
	return nil
}

// StoreBatch сохраняет множество метрик (для совместимости с интерфейсом)
func (r *MemStorage) StoreBatch(metrics []model.Metrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range metrics {
		m := &metrics[i]

		// Для counter нужно аккумулировать
		if m.MType == model.Counter {
			if existing, exists := r.data[m.ID]; exists && existing.MType == model.Counter && existing.Delta != nil && m.Delta != nil {
				newDelta := *existing.Delta + *m.Delta
				m.Delta = &newDelta
			}
		}

		r.data[m.ID] = m
	}

	return nil
}
