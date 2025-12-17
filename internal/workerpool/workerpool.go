package workerpool

import (
	"context"
	"log"
	"sync"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/retry"
)

// Job представляет задачу для воркера
type Job interface{}

// JobHandler обрабатывает задачу
type JobHandler func(ctx context.Context, job Job) error

// WorkerPool представляет пул воркеров
type WorkerPool struct {
	workers     int
	jobs        chan Job
	handler     JobHandler
	wg          sync.WaitGroup
	retryConfig retry.Config
	stopOnce    sync.Once // Защита от повторного закрытия
	stopped     bool
	mu          sync.RWMutex
}

// Config содержит конфигурацию worker pool
type Config struct {
	Workers     int
	BufferSize  int
	RetryConfig retry.Config
}

// New создает новый worker pool
func New(cfg Config, handler JobHandler) *WorkerPool {
	return &WorkerPool{
		workers:     cfg.Workers,
		jobs:        make(chan Job, cfg.BufferSize),
		handler:     handler,
		retryConfig: cfg.RetryConfig,
		stopped:     false,
	}
}

// Start запускает все воркеры
func (wp *WorkerPool) Start(ctx context.Context) {
	log.Printf("Starting worker pool with %d workers", wp.workers)

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i+1)
	}
}

// Submit отправляет задачу в пул (non-blocking)
func (wp *WorkerPool) Submit(job Job) bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if wp.stopped {
		return false
	}

	select {
	case wp.jobs <- job:
		return true
	default:
		return false
	}
}

// Stop останавливает worker pool и ждет завершения всех воркеров
func (wp *WorkerPool) Stop() {
	wp.stopOnce.Do(func() {
		log.Println("Stopping worker pool...")

		wp.mu.Lock()
		wp.stopped = true
		close(wp.jobs)
		wp.mu.Unlock()

		wp.wg.Wait()
		log.Println("Worker pool stopped")
	})
}

// worker обрабатывает задачи из канала
func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()

	log.Printf("Worker %d started", id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d: context cancelled", id)
			return

		case job, ok := <-wp.jobs:
			if !ok {
				log.Printf("Worker %d: channel closed", id)
				return
			}

			// Обрабатываем задачу с retry и поддержкой контекста
			err := retry.DoWithContext(ctx, func(ctx context.Context) error {
				return wp.handler(ctx, job)
			}, wp.retryConfig)

			if err != nil {
				if ctx.Err() != nil {
					log.Printf("Worker %d: job cancelled due to context: %v", id, ctx.Err())
				} else {
					log.Printf("Worker %d: failed to process job: %v", id, err)
				}
			}
		}
	}
}

// Jobs возвращает канал для прямого доступа (для обратной совместимости)
func (wp *WorkerPool) Jobs() chan<- Job {
	return wp.jobs
}

// IsStopped проверяет, остановлен ли worker pool
func (wp *WorkerPool) IsStopped() bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.stopped
}
