package service

import (
	"errors"
	"testing"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
)

func TestUpdateMetrics_InvalidType(t *testing.T) {
	svc := New(repository.New())
	err := svc.UpdateMetrics("unknown", "m", "1")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidMetricType) {
		t.Fatalf("expected ErrInvalidMetricType, got %v", err)
	}
}

func TestUpdateMetrics_CounterAccumulation(t *testing.T) {
	svc := New(repository.New())

	if err := svc.UpdateMetrics(model.Counter, "PollCount", "10"); err != nil {
		t.Fatalf("first update failed: %v", err)
	}
	if err := svc.UpdateMetrics(model.Counter, "PollCount", "5"); err != nil {
		t.Fatalf("second update failed: %v", err)
	}

	m, ok := svc.GetMetric("PollCount")
	if !ok || m == nil || m.Delta == nil {
		t.Fatalf("expected stored counter metric")
	}
	if got, want := *m.Delta, int64(15); got != want {
		t.Fatalf("delta: got %d want %d", got, want)
	}
}

func TestUpdateMetrics_GaugeOverwrite(t *testing.T) {
	svc := New(repository.New())

	if err := svc.UpdateMetrics(model.Gauge, "Alloc", "1.5"); err != nil {
		t.Fatalf("first update failed: %v", err)
	}
	if err := svc.UpdateMetrics(model.Gauge, "Alloc", "2.25"); err != nil {
		t.Fatalf("second update failed: %v", err)
	}

	m, ok := svc.GetMetric("Alloc")
	if !ok || m == nil || m.Value == nil {
		t.Fatalf("expected stored gauge metric")
	}
	if got, want := *m.Value, 2.25; got != want {
		t.Fatalf("value: got %v want %v", got, want)
	}
	if m.Delta != nil {
		t.Fatalf("expected delta nil for gauge")
	}
}

func TestUpdateMetrics_InvalidCounterValue(t *testing.T) {
	svc := New(repository.New())
	err := svc.UpdateMetrics(model.Counter, "PollCount", "not-a-number")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidCounterValue) {
		t.Fatalf("expected ErrInvalidCounterValue, got %v", err)
	}
}

func TestUpdateMetrics_InvalidGaugeValue(t *testing.T) {
	svc := New(repository.New())
	err := svc.UpdateMetrics(model.Gauge, "Alloc", "not-a-float")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidGaugeValue) {
		t.Fatalf("expected ErrInvalidGaugeValue, got %v", err)
	}
}

func TestUpdateMetricsBatch_FallbackAndValidation(t *testing.T) {
	svc := New(&metricRepoStub{data: map[string]*model.Metrics{}})

	metrics := []model.Metrics{
		{ID: "G1", MType: model.Gauge, Value: floatPtr(1.0)},
		{ID: "C1", MType: model.Counter, Delta: int64Ptr(2)},
	}

	if err := svc.UpdateMetricsBatch(metrics); err != nil {
		t.Fatalf("UpdateMetricsBatch failed: %v", err)
	}

	if _, ok := svc.GetMetric("G1"); !ok {
		t.Fatalf("expected G1 to be stored")
	}
	if _, ok := svc.GetMetric("C1"); !ok {
		t.Fatalf("expected C1 to be stored")
	}

	// invalid metric in batch: missing delta for counter (fallback path expects non-nil Delta)
	bad := []model.Metrics{{ID: "Bad", MType: model.Counter, Delta: nil}}
	if err := svc.UpdateMetricsBatch(bad); err == nil {
		t.Fatalf("expected error for bad batch metric")
	}
}

func floatPtr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64     { return &v }

type metricRepoStub struct {
	data map[string]*model.Metrics
}

func (r *metricRepoStub) Store(metric *model.Metrics) error {
	if r.data == nil {
		r.data = map[string]*model.Metrics{}
	}
	r.data[metric.ID] = metric
	return nil
}

func (r *metricRepoStub) Get(name string) (*model.Metrics, bool, error) {
	m, ok := r.data[name]
	return m, ok, nil
}

func (r *metricRepoStub) GetAll() (map[string]*model.Metrics, error) {
	out := make(map[string]*model.Metrics, len(r.data))
	for k, v := range r.data {
		out[k] = v
	}
	return out, nil
}

func (r *metricRepoStub) LoadData(data map[string]*model.Metrics) error {
	r.data = data
	return nil
}
