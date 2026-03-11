package audit

import (
	"context"
	"sync"
)

// Observer получает события аудита.
// Реализации должны быть неблокирующими и не падать наружу: аудит не должен ломать обработку метрик.
type Observer interface {
	OnAudit(ctx context.Context, e Event)
}

// Subject реализует паттерн "Наблюдатель": хранит подписчиков и рассылает события.
type Subject struct {
	mu        sync.RWMutex
	observers []Observer
}

func NewSubject() *Subject {
	return &Subject{}
}

func (s *Subject) Subscribe(o Observer) {
	if o == nil {
		return
	}
	s.mu.Lock()
	s.observers = append(s.observers, o)
	s.mu.Unlock()
}

func (s *Subject) HasObservers() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.observers) > 0
}

func (s *Subject) Notify(ctx context.Context, e Event) {
	s.mu.RLock()
	snapshot := make([]Observer, len(s.observers))
	copy(snapshot, s.observers)
	s.mu.RUnlock()

	for _, o := range snapshot {
		o.OnAudit(ctx, e)
	}
}
