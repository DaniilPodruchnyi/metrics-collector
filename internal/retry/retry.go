package retry

import (
	"context"
	"log"
	"time"
)

// Retryable определяет функцию, которую можно повторить
type Retryable func() error

// RetryableWithContext определяет функцию с контекстом, которую можно повторить
type RetryableWithContext func(ctx context.Context) error

// Config содержит конфигурацию для retry логики
type Config struct {
	MaxAttempts int
	Delays      []time.Duration
}

// DefaultConfig возвращает конфигурацию по умолчанию (3 попытки: 1s, 3s, 5s)
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		Delays:      []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second},
	}
}

// Do выполняет функцию с retry логикой
func Do(fn Retryable, cfg Config) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Последняя попытка - не ждем
		if attempt == cfg.MaxAttempts {
			break
		}

		// Получаем задержку для текущей попытки
		delay := cfg.Delays[attempt]
		log.Printf("Retry attempt %d/%d failed: %v. Retrying in %v...",
			attempt+1, cfg.MaxAttempts, err, delay)

		time.Sleep(delay)
	}

	return lastErr
}

// DoWithContext выполняет функцию с retry логикой и учетом контекста
func DoWithContext(ctx context.Context, fn RetryableWithContext, cfg Config) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxAttempts; attempt++ {
		// Проверяем контекст перед каждой попыткой
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Последняя попытка - не ждем
		if attempt == cfg.MaxAttempts {
			break
		}

		// Получаем задержку для текущей попытки
		delay := cfg.Delays[attempt]
		log.Printf("Retry attempt %d/%d failed: %v. Retrying in %v...",
			attempt+1, cfg.MaxAttempts, err, delay)

		// Ждем с учетом контекста
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

// IsRetryable проверяет, является ли ошибка retriable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Список retriable ошибок
	retriableErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"timeout",
		"temporary failure",
		"no such host",
		"network is unreachable",
	}

	errStr := err.Error()
	for _, retryable := range retriableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
