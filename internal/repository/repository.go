package repository

import (
	"sync"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
)

// MetricsRepository определяет интерфейс для работы с хранилищем метрик
type MetricRepository interface {
	Store(metric *model.Metrics)
	Get(name string) (*model.Metrics, bool)
	GetAll() map[string]*model.Metrics
	LoadData(data map[string]*model.Metrics)
}

// Структура для работы с хранилищем данных метрик
type MemStorage struct {
	data map[string]*model.Metrics
	mu   sync.RWMutex // Добавляем мьютекс для безопасности
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
func (r *MemStorage) Store(metric *model.Metrics) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[metric.ID] = metric
}

// Get возвращает метрику по имени
func (r *MemStorage) Get(name string) (*model.Metrics, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	metric, ok := r.data[name]
	return metric, ok
}

// GetAll возвращает все метрики
func (r *MemStorage) GetAll() map[string]*model.Metrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*model.Metrics, len(r.data))
	for k, v := range r.data {
		result[k] = v
	}
	return result
}

// LoadData загружает данные в хранилище (для восстановления из файла)
func (r *MemStorage) LoadData(data map[string]*model.Metrics) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = data
}
