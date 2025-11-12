package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
)

func TestFileStorage_SaveAndLoad(t *testing.T) {
	// Создаем временную директорию
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "metrics.json")

	fs := NewFileStorage(filePath)

	// Подготавливаем тестовые данные
	metrics := map[string]*model.Metrics{
		"Alloc": {
			ID:    "Alloc",
			MType: model.Gauge,
			Value: floatPtr(123.456),
		},
		"PollCount": {
			ID:    "PollCount",
			MType: model.Counter,
			Delta: int64Ptr(42),
		},
	}

	// Сохраняем
	err := fs.Save(metrics)
	if err != nil {
		t.Fatalf("Failed to save metrics: %v", err)
	}

	// Проверяем, что файл создан
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("File was not created")
	}

	// Загружаем
	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Failed to load metrics: %v", err)
	}

	// Проверяем количество метрик
	if len(loaded) != len(metrics) {
		t.Errorf("Expected %d metrics, got %d", len(metrics), len(loaded))
	}

	// Проверяем gauge метрику
	if metric, ok := loaded["Alloc"]; !ok {
		t.Error("Alloc metric not found")
	} else {
		if metric.MType != model.Gauge {
			t.Errorf("Expected type gauge, got %s", metric.MType)
		}
		if metric.Value == nil || *metric.Value != 123.456 {
			t.Errorf("Expected value 123.456, got %v", metric.Value)
		}
	}

	// Проверяем counter метрику
	if metric, ok := loaded["PollCount"]; !ok {
		t.Error("PollCount metric not found")
	} else {
		if metric.MType != model.Counter {
			t.Errorf("Expected type counter, got %s", metric.MType)
		}
		if metric.Delta == nil || *metric.Delta != 42 {
			t.Errorf("Expected delta 42, got %v", metric.Delta)
		}
	}
}

func TestFileStorage_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.json")

	fs := NewFileStorage(filePath)

	// Загрузка несуществующего файла должна вернуть пустую map
	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(loaded) != 0 {
		t.Errorf("Expected empty map, got %d metrics", len(loaded))
	}
}

func TestFileStorage_SaveEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.json")

	fs := NewFileStorage(filePath)

	// Сохраняем пустую map
	err := fs.Save(make(map[string]*model.Metrics))
	if err != nil {
		t.Fatalf("Failed to save empty metrics: %v", err)
	}

	// Загружаем
	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Failed to load metrics: %v", err)
	}

	if len(loaded) != 0 {
		t.Errorf("Expected 0 metrics, got %d", len(loaded))
	}
}

func floatPtr(f float64) *float64 { return &f }
func int64Ptr(i int64) *int64     { return &i }
