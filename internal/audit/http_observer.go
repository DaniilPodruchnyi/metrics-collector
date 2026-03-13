package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

// HTTPObserver отправляет события аудита на удаленный HTTP-сервис.
type HTTPObserver struct {
	url    string
	client *http.Client
}

var ErrEmptyAuditURL = errors.New("empty audit url")

// NewHTTPObserver создает HTTP-обработчик аудита.
// При некорректных параметрах возвращает ошибку.
func NewHTTPObserver(url string) (*HTTPObserver, error) {
	if url == "" {
		return nil, ErrEmptyAuditURL
	}
	return &HTTPObserver{
		url: url,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}, nil
}

func (o *HTTPObserver) OnAudit(_ context.Context, e Event) {
	b, err := json.Marshal(e)
	if err != nil {
		log.Printf("audit(http): marshal failed: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, o.url, bytes.NewReader(b))
	if err != nil {
		log.Printf("audit(http): new request failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		log.Printf("audit(http): post failed: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("audit(http): non-2xx response: %s", resp.Status)
		return
	}
}
