package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
)

// PersistentStorage - интерфейс для персистентного хранения метрик
type PersistentStorage interface {
	Save(metrics map[string]*model.Metrics) error
	Load() (map[string]*model.Metrics, error)
}

// FileStorage обрабатывает персистентность метрик в файл
type FileStorage struct {
	filePath string
}

// Проверяем, что FileStorage реализует PersistentStorage
var _ PersistentStorage = (*FileStorage)(nil)

// NewFileStorage создает новый FileStorage
func NewFileStorage(filePath string) *FileStorage {
	return &FileStorage{
		filePath: filePath,
	}
}

// Save сохраняет метрики в файл атомарно
func (fs *FileStorage) Save(metrics map[string]*model.Metrics) error {
	// Преобразуем map в slice для JSON
	metricsList := make([]*model.Metrics, 0, len(metrics))
	for _, metric := range metrics {
		metricsList = append(metricsList, metric)
	}

	// Сериализуем в JSON с отступами для читаемости
	data, err := json.MarshalIndent(metricsList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Атомарная запись: сначала во временный файл
	dir := filepath.Dir(fs.filePath)
	tmpFile, err := os.CreateTemp(dir, "metrics-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Записываем данные
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Синхронизируем на диск
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	tmpFile.Close()

	// Атомарно переименовываем (rename - atomic operation on POSIX)
	if err := os.Rename(tmpPath, fs.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Load загружает метрики из файла
func (fs *FileStorage) Load() (map[string]*model.Metrics, error) {
	// Проверяем существование файла
	if _, err := os.Stat(fs.filePath); os.IsNotExist(err) {
		return make(map[string]*model.Metrics), nil
	}

	// Читаем файл
	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Десериализуем из JSON
	var metricsList []*model.Metrics
	if err := json.Unmarshal(data, &metricsList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	// Преобразуем slice в map
	metricsMap := make(map[string]*model.Metrics, len(metricsList))
	for _, metric := range metricsList {
		metricsMap[metric.ID] = metric
	}

	return metricsMap, nil
}
