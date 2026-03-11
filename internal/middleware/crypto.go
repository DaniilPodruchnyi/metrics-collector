package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"log"
	"net/http"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/security"
)

// CryptoMiddleware расшифровывает тело запроса, если оно помечено как зашифрованное.
// Должен вызываться после GzipMiddleware (который в этом случае просто ничего не делает,
// так как Content-Encoding не установлен) и до HashVerificationMiddleware.
func CryptoMiddleware(privKey *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Если ключ не настроен — шифрование отключено
			if privKey == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Проверяем, помечен ли запрос как зашифрованный
			if r.Header.Get("X-Encrypted") != "rsa" {
				next.ServeHTTP(w, r)
				return
			}

			// Читаем зашифрованное тело
			cipherBody, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("Failed to read encrypted request body: %v", err)
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}

			// Восстанавливаем тело, чтобы в случае ошибки можно было логировать размер
			r.Body = io.NopCloser(bytes.NewReader(cipherBody))

			// Расшифровываем
			plainBody, err := security.DecryptRSA(cipherBody, privKey)
			if err != nil {
				log.Printf("Failed to decrypt request body: %v", err)
				http.Error(w, "Failed to decrypt request", http.StatusBadRequest)
				return
			}

			// Подменяем тело расшифрованными данными для следующих обработчиков
			r.Body = io.NopCloser(bytes.NewReader(plainBody))

			next.ServeHTTP(w, r)
		})
	}
}

