package repository

import (
	"sync"
	"testing"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
)

// Вспомогательные функции
func floatPtr(f float64) *float64 { return &f }
func int64Ptr(i int64) *int64     { return &i }

func TestNew(t *testing.T) {
	repo := New()

	if repo == nil {
		t.Fatal("Expected non-nil repository")
	}

	if repo.data == nil {
		t.Fatal("Expected initialized data map")
	}

	if len(repo.data) != 0 {
		t.Errorf("Expected empty repository, got %d items", len(repo.data))
	}
}

func TestNewWithData(t *testing.T) {
	data := map[string]*model.Metrics{
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

	repo := NewWithData(data)

	if repo == nil {
		t.Fatal("Expected non-nil repository")
	}

	if len(repo.data) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(repo.data))
	}

	// Проверяем, что данные корректно загружены
	if metric, ok := repo.data["Alloc"]; !ok {
		t.Error("Alloc metric not found")
	} else if *metric.Value != 123.456 {
		t.Errorf("Expected value 123.456, got %v", *metric.Value)
	}
}

func TestStore(t *testing.T) {
	repo := New()

	metric := &model.Metrics{
		ID:    "TestGauge",
		MType: model.Gauge,
		Value: floatPtr(100.5),
	}

	err := repo.Store(metric)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Проверяем, что метрика сохранена
	stored, ok := repo.data["TestGauge"]
	if !ok {
		t.Fatal("Metric was not stored")
	}

	if stored.ID != "TestGauge" {
		t.Errorf("Expected ID TestGauge, got %s", stored.ID)
	}

	if *stored.Value != 100.5 {
		t.Errorf("Expected value 100.5, got %v", *stored.Value)
	}
}

func TestStore_Overwrite(t *testing.T) {
	repo := New()

	// Первая запись
	metric1 := &model.Metrics{
		ID:    "TestMetric",
		MType: model.Gauge,
		Value: floatPtr(100.0),
	}
	if err := repo.Store(metric1); err != nil {
		t.Fatalf("First store failed: %v", err)
	}

	// Перезапись
	metric2 := &model.Metrics{
		ID:    "TestMetric",
		MType: model.Gauge,
		Value: floatPtr(200.0),
	}
	if err := repo.Store(metric2); err != nil {
		t.Fatalf("Second store failed: %v", err)
	}

	// Проверяем, что значение обновилось
	stored := repo.data["TestMetric"]
	if *stored.Value != 200.0 {
		t.Errorf("Expected value 200.0, got %v", *stored.Value)
	}

	// Проверяем, что в хранилище только одна метрика
	if len(repo.data) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(repo.data))
	}
}

func TestGet(t *testing.T) {
	repo := New()

	// Добавляем метрику
	metric := &model.Metrics{
		ID:    "TestCounter",
		MType: model.Counter,
		Delta: int64Ptr(42),
	}
	if err := repo.Store(metric); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Получаем метрику
	retrieved, ok, err := repo.Get("TestCounter")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !ok {
		t.Fatal("Metric not found")
	}

	if retrieved.ID != "TestCounter" {
		t.Errorf("Expected ID TestCounter, got %s", retrieved.ID)
	}

	if *retrieved.Delta != 42 {
		t.Errorf("Expected delta 42, got %v", *retrieved.Delta)
	}
}

func TestGet_NotFound(t *testing.T) {
	repo := New()

	// Пытаемся получить несуществующую метрику
	_, ok, err := repo.Get("NonExistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if ok {
		t.Error("Expected metric to not be found")
	}
}

func TestGetAll(t *testing.T) {
	repo := New()

	// Добавляем несколько метрик
	metrics := []*model.Metrics{
		{
			ID:    "Metric1",
			MType: model.Gauge,
			Value: floatPtr(1.0),
		},
		{
			ID:    "Metric2",
			MType: model.Counter,
			Delta: int64Ptr(2),
		},
		{
			ID:    "Metric3",
			MType: model.Gauge,
			Value: floatPtr(3.0),
		},
	}

	for _, metric := range metrics {
		if err := repo.Store(metric); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	// Получаем все метрики
	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("Expected 3 metrics, got %d", len(all))
	}

	// Проверяем наличие всех метрик
	for _, metric := range metrics {
		if _, ok := all[metric.ID]; !ok {
			t.Errorf("Metric %s not found in GetAll", metric.ID)
		}
	}
}

func TestGetAll_Empty(t *testing.T) {
	repo := New()

	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(all) != 0 {
		t.Errorf("Expected 0 metrics, got %d", len(all))
	}
}

func TestGetAll_ReturnsIndependentCopy(t *testing.T) {
	repo := New()

	metric := &model.Metrics{
		ID:    "Test",
		MType: model.Gauge,
		Value: floatPtr(1.0),
	}
	if err := repo.Store(metric); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Получаем копию
	all1, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	// Изменяем возвращенную map
	all1["NewMetric"] = &model.Metrics{
		ID:    "NewMetric",
		MType: model.Gauge,
		Value: floatPtr(2.0),
	}

	// Получаем еще раз
	all2, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	// Проверяем, что внутренние данные не изменились
	if len(all2) != 1 {
		t.Errorf("Expected 1 metric in repository, got %d", len(all2))
	}

	if _, ok := all2["NewMetric"]; ok {
		t.Error("Internal data was modified, expected independence")
	}
}

func TestLoadData(t *testing.T) {
	repo := New()

	// Добавляем начальные данные
	if err := repo.Store(&model.Metrics{
		ID:    "Old",
		MType: model.Gauge,
		Value: floatPtr(1.0),
	}); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Загружаем новые данные
	newData := map[string]*model.Metrics{
		"New1": {
			ID:    "New1",
			MType: model.Gauge,
			Value: floatPtr(2.0),
		},
		"New2": {
			ID:    "New2",
			MType: model.Counter,
			Delta: int64Ptr(3),
		},
	}

	if err := repo.LoadData(newData); err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	// Проверяем, что старые данные заменены
	if _, ok, _ := repo.Get("Old"); ok {
		t.Error("Old data should be replaced")
	}

	// Проверяем новые данные
	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(all) != 2 {
		t.Errorf("Expected 2 metrics after LoadData, got %d", len(all))
	}

	if _, ok := all["New1"]; !ok {
		t.Error("New1 not found after LoadData")
	}

	if _, ok := all["New2"]; !ok {
		t.Error("New2 not found after LoadData")
	}
}

func TestLoadData_Empty(t *testing.T) {
	repo := New()

	// Добавляем данные
	if err := repo.Store(&model.Metrics{
		ID:    "Test",
		MType: model.Gauge,
		Value: floatPtr(1.0),
	}); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Загружаем пустую map
	if err := repo.LoadData(make(map[string]*model.Metrics)); err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(all) != 0 {
		t.Errorf("Expected 0 metrics after loading empty data, got %d", len(all))
	}
}

func TestStoreBatch(t *testing.T) {
	repo := New()

	metrics := []model.Metrics{
		{
			ID:    "Gauge1",
			MType: model.Gauge,
			Value: floatPtr(100.5),
		},
		{
			ID:    "Counter1",
			MType: model.Counter,
			Delta: int64Ptr(10),
		},
	}

	err := repo.StoreBatch(metrics)
	if err != nil {
		t.Fatalf("StoreBatch failed: %v", err)
	}

	// Проверяем, что метрики сохранены
	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(all) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(all))
	}

	gauge, ok := all["Gauge1"]
	if !ok {
		t.Error("Gauge1 not found")
	} else if *gauge.Value != 100.5 {
		t.Errorf("Expected gauge value 100.5, got %v", *gauge.Value)
	}

	counter, ok := all["Counter1"]
	if !ok {
		t.Error("Counter1 not found")
	} else if *counter.Delta != 10 {
		t.Errorf("Expected counter delta 10, got %v", *counter.Delta)
	}
}

func TestStoreBatch_CounterAccumulation(t *testing.T) {
	repo := New()

	// Первый batch
	batch1 := []model.Metrics{
		{
			ID:    "TestCounter",
			MType: model.Counter,
			Delta: int64Ptr(10),
		},
	}

	if err := repo.StoreBatch(batch1); err != nil {
		t.Fatalf("First StoreBatch failed: %v", err)
	}

	// Второй batch (должен добавиться к существующему)
	batch2 := []model.Metrics{
		{
			ID:    "TestCounter",
			MType: model.Counter,
			Delta: int64Ptr(5),
		},
	}

	if err := repo.StoreBatch(batch2); err != nil {
		t.Fatalf("Second StoreBatch failed: %v", err)
	}

	// Проверяем итоговое значение
	metric, ok, err := repo.Get("TestCounter")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !ok {
		t.Fatal("Counter not found")
	}

	if *metric.Delta != 15 {
		t.Errorf("Expected counter value 15, got %d", *metric.Delta)
	}
}

// Тесты на thread-safety
func TestStore_Concurrent(t *testing.T) {
	repo := New()
	var wg sync.WaitGroup

	// Запускаем 100 горутин, каждая записывает 100 метрик
	numGoroutines := 100
	numOperations := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				metric := &model.Metrics{
					ID:    "Metric",
					MType: model.Counter,
					Delta: int64Ptr(int64(id*numOperations + j)),
				}
				repo.Store(metric)
			}
		}(i)
	}

	wg.Wait()

	// Проверяем, что метрика сохранена (любое значение)
	_, ok, err := repo.Get("Metric")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !ok {
		t.Error("Expected metric to be stored")
	}
}

func TestGet_Concurrent(t *testing.T) {
	repo := New()

	// Добавляем метрики
	for i := 0; i < 10; i++ {
		metric := &model.Metrics{
			ID:    "Metric" + string(rune('0'+i)),
			MType: model.Gauge,
			Value: floatPtr(float64(i)),
		}
		if err := repo.Store(metric); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				repo.Get("Metric0")
			}
		}()
	}

	wg.Wait()
}

func TestGetAll_Concurrent(t *testing.T) {
	repo := New()

	// Добавляем метрики
	for i := 0; i < 10; i++ {
		metric := &model.Metrics{
			ID:    "Metric" + string(rune('0'+i)),
			MType: model.Gauge,
			Value: floatPtr(float64(i)),
		}
		if err := repo.Store(metric); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				repo.GetAll()
			}
		}()
	}

	wg.Wait()
}

func TestMixedOperations_Concurrent(t *testing.T) {
	repo := New()
	var wg sync.WaitGroup

	numGoroutines := 50

	// Горутины для записи
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				metric := &model.Metrics{
					ID:    "Metric" + string(rune('0'+id)),
					MType: model.Gauge,
					Value: floatPtr(float64(j)),
				}
				repo.Store(metric)
			}
		}(i)
	}

	// Горутины для чтения
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				repo.Get("Metric" + string(rune('0'+id)))
				repo.GetAll()
			}
		}(i)
	}

	wg.Wait()

	// Проверяем, что данные корректны
	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(all) == 0 {
		t.Error("Expected some metrics to be stored")
	}
}

func TestLoadData_Concurrent(t *testing.T) {
	repo := New()
	var wg sync.WaitGroup

	numGoroutines := 10

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			data := map[string]*model.Metrics{
				"Metric": {
					ID:    "Metric",
					MType: model.Counter,
					Delta: int64Ptr(int64(id)),
				},
			}
			repo.LoadData(data)
		}(i)
	}

	wg.Wait()

	// Проверяем, что метрика существует (любое значение)
	_, ok, err := repo.Get("Metric")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !ok {
		t.Error("Expected metric to be present after concurrent LoadData")
	}
}

// Бенчмарки
func BenchmarkStore(b *testing.B) {
	repo := New()
	metric := &model.Metrics{
		ID:    "BenchMetric",
		MType: model.Gauge,
		Value: floatPtr(123.456),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.Store(metric)
	}
}

func BenchmarkGet(b *testing.B) {
	repo := New()
	metric := &model.Metrics{
		ID:    "BenchMetric",
		MType: model.Gauge,
		Value: floatPtr(123.456),
	}
	repo.Store(metric)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.Get("BenchMetric")
	}
}

func BenchmarkGetAll(b *testing.B) {
	repo := New()

	// Добавляем 100 метрик
	for i := 0; i < 100; i++ {
		metric := &model.Metrics{
			ID:    "Metric" + string(rune('0'+i)),
			MType: model.Gauge,
			Value: floatPtr(float64(i)),
		}
		repo.Store(metric)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.GetAll()
	}
}

func BenchmarkStore_Concurrent(b *testing.B) {
	repo := New()
	metric := &model.Metrics{
		ID:    "BenchMetric",
		MType: model.Gauge,
		Value: floatPtr(123.456),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			repo.Store(metric)
		}
	})
}

func BenchmarkGet_Concurrent(b *testing.B) {
	repo := New()
	metric := &model.Metrics{
		ID:    "BenchMetric",
		MType: model.Gauge,
		Value: floatPtr(123.456),
	}
	repo.Store(metric)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			repo.Get("BenchMetric")
		}
	})
}
