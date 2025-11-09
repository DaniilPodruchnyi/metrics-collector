package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Вспомогательная функция для сжатия данных
func gzipCompress(data []byte) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

// Вспомогательная функция для распаковки данных
func gzipDecompress(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return io.ReadAll(gz)
}

func TestGzipMiddleware_Compression(t *testing.T) {
	// Создаем тестовый handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"hello world"}`))
	})

	// Оборачиваем в middleware
	wrappedHandler := GzipMiddleware(handler)

	// Создаем запрос с поддержкой gzip
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	// Проверяем заголовки
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip, got: %s", w.Header().Get("Content-Encoding"))
	}

	// Проверяем, что данные действительно сжаты
	decompressed, err := gzipDecompress(w.Body.Bytes())
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	expected := `{"message":"hello world"}`
	if string(decompressed) != expected {
		t.Errorf("Expected %q, got %q", expected, string(decompressed))
	}
}

func TestGzipMiddleware_NoCompression(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"hello world"}`))
	})

	wrappedHandler := GzipMiddleware(handler)

	// Запрос БЕЗ поддержки gzip
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	// Проверяем, что Content-Encoding не установлен
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Expected no gzip compression")
	}

	// Проверяем, что данные НЕ сжаты
	expected := `{"message":"hello world"}`
	if w.Body.String() != expected {
		t.Errorf("Expected %q, got %q", expected, w.Body.String())
	}
}

func TestGzipMiddleware_Decompression(t *testing.T) {
	var receivedBody string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := GzipMiddleware(handler)

	// Сжимаем тело запроса
	originalData := []byte(`{"metric":"test","value":123}`)
	compressed, err := gzipCompress(originalData)
	if err != nil {
		t.Fatalf("Failed to compress data: %v", err)
	}

	// Создаем запрос со сжатым телом
	req := httptest.NewRequest(http.MethodPost, "/", compressed)
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	// Проверяем, что тело было распаковано
	if receivedBody != string(originalData) {
		t.Errorf("Expected %q, got %q", string(originalData), receivedBody)
	}
}

func TestGzipMiddleware_InvalidGzip(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := GzipMiddleware(handler)

	// Отправляем невалидные gzip данные
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("invalid gzip data"))
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	// Должна вернуться ошибка 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGzipMiddleware_HTMLContent(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Hello</body></html>"))
	})

	wrappedHandler := GzipMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Expected HTML content to be compressed")
	}

	decompressed, err := gzipDecompress(w.Body.Bytes())
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	expected := "<html><body>Hello</body></html>"
	if string(decompressed) != expected {
		t.Errorf("Expected %q, got %q", expected, string(decompressed))
	}
}

func TestShouldCompress(t *testing.T) {
	tests := []struct {
		contentType string
		want        bool
	}{
		{"application/json", true},
		{"text/html", true},
		{"text/plain", true},
		{"text/css", true},
		{"application/javascript", true},
		{"image/png", false},
		{"video/mp4", false},
		{"application/octet-stream", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			got := shouldCompress(tt.contentType)
			if got != tt.want {
				t.Errorf("shouldCompress(%q) = %v, want %v", tt.contentType, got, tt.want)
			}
		})
	}
}
