package pool

import "sync"

// Resetter describes types that can reset their internal state.
type Resetter interface {
	Reset()
}

// Pool is a generic container for objects of a single concrete type
// that implements the Reset method.
type Pool[T Resetter] struct {
	p sync.Pool
}

// New creates a new Pool for objects of type T.
// newFn is used to create new instances when the pool is empty.
func New[T Resetter](newFn func() T) *Pool[T] {
	return &Pool[T]{
		p: sync.Pool{
			New: func() any {
				return newFn()
			},
		},
	}
}

// Get returns an object from the pool.
func (p *Pool[T]) Get() T {
	v := p.p.Get()
	if v == nil {
		var zero T
		return zero
	}
	return v.(T)
}

// Put resets the object and returns it to the pool.
func (p *Pool[T]) Put(v T) {
	if any(v) == nil {
		return
	}
	v.Reset()
	p.p.Put(v)
}

