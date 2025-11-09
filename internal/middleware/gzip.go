package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// gzipWriter оборачивает http.ResponseWriter для сжатия ответа
type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

// Write переопределяет метод Write для записи сжатых данных
func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// GzipMiddleware обрабатывает gzip compression/decompression
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем, поддерживает ли клиент gzip
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")

		// Декомпрессия входящего запроса, если он сжат
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Failed to decompress request body", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			defer r.Body.Close()
			r.Body = gz
		}

		// Если клиент поддерживает gzip и Content-Type подходит для сжатия
		if supportsGzip {
			// Создаем gzip writer для ответа
			gz := gzip.NewWriter(w)
			defer gz.Close()

			// Оборачиваем ResponseWriter
			gzw := gzipWriter{
				ResponseWriter: w,
				Writer:         gz,
			}

			// Устанавливаем заголовок Content-Encoding
			w.Header().Set("Content-Encoding", "gzip")

			// Удаляем Content-Length, так как размер изменится после сжатия
			w.Header().Del("Content-Length")

			// Передаем управление следующему обработчику с wrapped writer
			next.ServeHTTP(gzw, r)
			return
		}

		// Если gzip не поддерживается, просто передаем управление дальше
		next.ServeHTTP(w, r)
	})
}

// shouldCompress проверяет, нужно ли сжимать контент на основе Content-Type
func shouldCompress(contentType string) bool {
	compressibleTypes := []string{
		"application/json",
		"text/html",
		"text/plain",
		"text/css",
		"text/javascript",
		"application/javascript",
	}

	for _, ct := range compressibleTypes {
		if strings.Contains(contentType, ct) {
			return true
		}
	}
	return false
}

// GzipMiddlewareWithFilter - улучшенная версия с фильтром по Content-Type
func GzipMiddlewareWithFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Декомпрессия входящего запроса
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Failed to decompress request body", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			defer r.Body.Close()
			r.Body = gz
		}

		// Проверяем поддержку gzip клиентом
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")

		if !supportsGzip {
			next.ServeHTTP(w, r)
			return
		}

		// Создаем перехватчик для проверки Content-Type перед сжатием
		interceptor := &responseInterceptor{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(interceptor, r)

		// Проверяем Content-Type после выполнения handler
		contentType := interceptor.Header().Get("Content-Type")

		if shouldCompress(contentType) && len(interceptor.body) > 0 {
			// Сжимаем ответ
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")
			w.WriteHeader(interceptor.statusCode)

			gz := gzip.NewWriter(w)
			defer gz.Close()
			gz.Write(interceptor.body)
		} else {
			// Отправляем несжатый ответ
			w.WriteHeader(interceptor.statusCode)
			w.Write(interceptor.body)
		}
	})
}

// responseInterceptor перехватывает ответ для проверки Content-Type
type responseInterceptor struct {
	http.ResponseWriter
	body       []byte
	statusCode int
}

func (ri *responseInterceptor) Write(b []byte) (int, error) {
	ri.body = append(ri.body, b...)
	return len(b), nil
}

func (ri *responseInterceptor) WriteHeader(statusCode int) {
	ri.statusCode = statusCode
}
