package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil
	}

	cfg := DefaultConfig()
	err := Do(fn, cfg)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestDo_MaxAttemptsExceeded(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("persistent error")
	}

	cfg := DefaultConfig()
	err := Do(fn, cfg)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attempts != 4 { // 1 original + 3 retries
		t.Errorf("Expected 4 attempts, got %d", attempts)
	}
}

func TestDoWithContext_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts == 2 {
			cancel()
		}
		return errors.New("error")
	}

	cfg := Config{
		MaxAttempts: 5,
		Delays:      []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond},
	}

	err := DoWithContext(ctx, fn, cfg)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection refused", errors.New("connection refused"), true},
		{"timeout", errors.New("timeout"), true},
		{"broken pipe", errors.New("broken pipe"), true},
		{"non-retriable", errors.New("syntax error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
