package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	status      int
	size        int
	wroteHeader bool
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	if !lrw.wroteHeader {
		lrw.status = code
		lrw.ResponseWriter.WriteHeader(code)
		lrw.wroteHeader = true
	}
}
func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if !lrw.wroteHeader {
		lrw.WriteHeader(http.StatusOK)
	}
	n, err := lrw.ResponseWriter.Write(b)
	lrw.size += n
	return n, err
}

func ZapLoggerMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lrw := &loggingResponseWriter{ResponseWriter: w}

			next.ServeHTTP(lrw, r)

			duration := time.Since(start)
			logger.Info("HTTP request",
				zap.String("method", r.Method),
				zap.String("uri", r.RequestURI),
				zap.Int("status", lrw.status),
				zap.Int("size", lrw.size),
				zap.Duration("duration", duration),
			)
		})
	}
}
