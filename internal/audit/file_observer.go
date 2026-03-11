package audit

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
)

// FileObserver пишет события аудита в указанный файл, добавляя по строке на событие.
type FileObserver struct {
	path string
	mu   sync.Mutex
}

// NewFileObserver создает файловый обработчик аудита; при пустом пути возвращает nil.
func NewFileObserver(path string) *FileObserver {
	if path == "" {
		return nil
	}
	return &FileObserver{path: path}
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
