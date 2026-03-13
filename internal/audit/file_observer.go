package audit

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
)

// FileObserver пишет события аудита в указанный файл, добавляя по строке на событие.
type FileObserver struct {
	path string
	mu   sync.Mutex
}

var ErrEmptyAuditFilePath = errors.New("empty audit file path")

// NewFileObserver создает файловый обработчик аудита.
// При некорректных параметрах возвращает ошибку.
func NewFileObserver(path string) (*FileObserver, error) {
	if path == "" {
		return nil, ErrEmptyAuditFilePath
	}
	return &FileObserver{path: path}, nil
}

func (o *FileObserver) OnAudit(_ context.Context, e Event) {
	b, err := json.Marshal(e)
	if err != nil {
		log.Printf("audit(file): marshal failed: %v", err)
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	f, err := os.OpenFile(o.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("audit(file): open failed: %v", err)
		return
	}
	defer func() {
		_ = f.Close()
	}()

	if _, err := f.Write(append(b, '\n')); err != nil {
		log.Printf("audit(file): write failed: %v", err)
		return
	}
}
