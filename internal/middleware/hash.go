package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/security"
)

// HashVerificationMiddleware проверяет HMAC-SHA256 подпись запроса
func HashVerificationMiddleware(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Если ключ не задан, пропускаем проверку
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Читаем тело запроса
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("Failed to read request body: %v", err)
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}

			// Распаковываем gzip если есть
			var originalBody []byte
			if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
				gz, err := gzip.NewReader(bytes.NewReader(body))
				if err != nil {
					log.Printf("Failed to create gzip reader: %v", err)
					http.Error(w, "Failed to decompress request", http.StatusBadRequest)
					return
				}
				defer gz.Close()

				originalBody, err = io.ReadAll(gz)
				if err != nil {
					log.Printf("Failed to decompress request body: %v", err)
					http.Error(w, "Failed to decompress request", http.StatusBadRequest)
					return
				}
			} else {
				originalBody = body
			}

			// Восстанавливаем тело для следующих обработчиков (сжатое)
			r.Body = io.NopCloser(bytes.NewReader(body))

			// Получаем хеш из заголовка
			receivedHash := r.Header.Get("HashSHA256")

			// Если хеш не передан, но ключ задан - пропускаем для обратной совместимости
			if receivedHash == "" {
				log.Println("Hash not provided in request, but key is configured")
				next.ServeHTTP(w, r)
				return
			}

			// Проверяем подпись НЕСЖАТЫХ данных
			if !security.VerifyHMAC(originalBody, key, receivedHash) {
				expectedHash := security.ComputeHMAC(originalBody, key)
				log.Printf("Hash verification failed. Expected: %s, Received: %s",
					expectedHash[:16]+"...", receivedHash[:16]+"...")
				http.Error(w, "Invalid signature", http.StatusBadRequest)
				return
			}

			log.Println("Hash verification successful")
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriterWithHash оборачивает ResponseWriter для подписи ответа
type responseWriterWithHash struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	key        string
}

func (rw *responseWriterWithHash) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriterWithHash) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// HashSigningMiddleware подписывает ответы сервера
func HashSigningMiddleware(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Если ключ не задан, пропускаем подпись
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Оборачиваем ResponseWriter
			wrapped := &responseWriterWithHash{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				statusCode:     http.StatusOK,
				key:            key,
			}

			// Вызываем следующий обработчик
			next.ServeHTTP(wrapped, r)

			// Подписываем ответ только для успешных запросов
			if wrapped.statusCode >= 200 && wrapped.statusCode < 300 {
				hash := security.ComputeHMAC(wrapped.body.Bytes(), key)
				w.Header().Set("HashSHA256", hash)
				log.Printf("Response signed with hash: %s", hash)
			}
		})
	}
}
