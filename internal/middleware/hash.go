package middleware

import (
	"bytes"
	"io"
	"log"
	"net/http"

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

			// Восстанавливаем тело для следующих обработчиков
			r.Body = io.NopCloser(bytes.NewReader(body))

			// Получаем хеш из заголовка
			receivedHash := r.Header.Get("HashSHA256")

			// Если хеш не передан, но ключ задан - это ошибка
			if receivedHash == "" {
				log.Println("Hash not provided in request, but key is configured")
				// Можно пропустить для обратной совместимости или отклонить
				next.ServeHTTP(w, r)
				return
			}

			// Проверяем подпись
			if !security.VerifyHMAC(body, key, receivedHash) {
				log.Printf("Hash verification failed. Expected vs Received: %s vs %s",
					security.ComputeHMAC(body, key), receivedHash)
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
