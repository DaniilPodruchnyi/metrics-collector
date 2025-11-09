package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestZapLoggerMiddleware(t *testing.T) {
	// Создаем тестовый логгер с observer для захвата логов
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	// Тестовый handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Оборачиваем в middleware
	middleware := ZapLoggerMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/test?param=value", nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	wrappedHandler.ServeHTTP(w, req)

	// Проверяем, что лог был записан
	if logs.Len() != 1 {
		t.Fatalf("Expected 1 log entry, got %d", logs.Len())
	}

	// Получаем запись лога
	logEntry := logs.All()[0]

	// Проверяем поля лога
	fields := logEntry.ContextMap()

	if fields["method"] != "GET" {
		t.Errorf("Expected method GET, got %v", fields["method"])
	}

	if fields["uri"] != "/test?param=value" {
		t.Errorf("Expected uri /test?param=value, got %v", fields["uri"])
	}

	if fields["status"] != int64(200) {
		t.Errorf("Expected status 200, got %v", fields["status"])
	}

	if fields["size"] != int64(13) { // len("test response")
		t.Errorf("Expected size 13, got %v", fields["size"])
	}

	if _, ok := fields["duration"]; !ok {
		t.Error("Expected duration field to be present")
	}
}

func TestZapLoggerMiddleware_DifferentStatuses(t *testing.T) {
	tests := []struct {
		name           string
		handlerStatus  int
		handlerBody    string
		expectedStatus int
		expectedSize   int
	}{
		{
			name:           "200 OK",
			handlerStatus:  http.StatusOK,
			handlerBody:    "success",
			expectedStatus: 200,
			expectedSize:   7,
		},
		{
			name:           "201 Created",
			handlerStatus:  http.StatusCreated,
			handlerBody:    "created",
			expectedStatus: 201,
			expectedSize:   7,
		},
		{
			name:           "400 Bad Request",
			handlerStatus:  http.StatusBadRequest,
			handlerBody:    "bad request",
			expectedStatus: 400,
			expectedSize:   11,
		},
		{
			name:           "404 Not Found",
			handlerStatus:  http.StatusNotFound,
			handlerBody:    "not found",
			expectedStatus: 404,
			expectedSize:   9,
		},
		{
			name:           "500 Internal Server Error",
			handlerStatus:  http.StatusInternalServerError,
			handlerBody:    "error",
			expectedStatus: 500,
			expectedSize:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.InfoLevel)
			logger := zap.New(core)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.handlerStatus)
				w.Write([]byte(tt.handlerBody))
			})

			middleware := ZapLoggerMiddleware(logger)
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if logs.Len() != 1 {
				t.Fatalf("Expected 1 log entry, got %d", logs.Len())
			}

			fields := logs.All()[0].ContextMap()

			if fields["status"] != int64(tt.expectedStatus) {
				t.Errorf("Expected status %d, got %v", tt.expectedStatus, fields["status"])
			}

			if fields["size"] != int64(tt.expectedSize) {
				t.Errorf("Expected size %d, got %v", tt.expectedSize, fields["size"])
			}
		})
	}
}

func TestZapLoggerMiddleware_DifferentMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			core, logs := observer.New(zap.InfoLevel)
			logger := zap.New(core)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := ZapLoggerMiddleware(logger)
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if logs.Len() != 1 {
				t.Fatalf("Expected 1 log entry, got %d", logs.Len())
			}

			fields := logs.All()[0].ContextMap()

			if fields["method"] != method {
				t.Errorf("Expected method %s, got %v", method, fields["method"])
			}
		})
	}
}

func TestZapLoggerMiddleware_EmptyResponse(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	// Handler с явной записью (чтобы триггернуть Write)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{}) // Пишем пустой массив байт
	})

	middleware := ZapLoggerMiddleware(logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if logs.Len() != 1 {
		t.Fatalf("Expected 1 log entry, got %d", logs.Len())
	}

	fields := logs.All()[0].ContextMap()

	// Статус должен быть 200
	if fields["status"] != int64(200) {
		t.Errorf("Expected status 200, got %v", fields["status"])
	}

	if fields["size"] != int64(0) {
		t.Errorf("Expected size 0, got %v", fields["size"])
	}
}

func TestZapLoggerMiddleware_MultipleWrites(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello "))
		w.Write([]byte("World"))
		w.Write([]byte("!"))
	})

	middleware := ZapLoggerMiddleware(logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if logs.Len() != 1 {
		t.Fatalf("Expected 1 log entry, got %d", logs.Len())
	}

	fields := logs.All()[0].ContextMap()

	// Должен посчитать общий размер всех записей
	expectedSize := len("Hello World!")
	if fields["size"] != int64(expectedSize) {
		t.Errorf("Expected size %d, got %v", expectedSize, fields["size"])
	}
}

func TestZapLoggerMiddleware_URIWithQueryParams(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ZapLoggerMiddleware(logger)
	wrappedHandler := middleware(handler)

	tests := []string{
		"/api/metrics?name=test&value=123",
		"/update/gauge/Alloc/123.456",
		"/",
		"/api/v1/resource?filter=active&sort=desc",
	}

	for _, uri := range tests {
		t.Run(uri, func(t *testing.T) {
			logs.TakeAll() // Очищаем предыдущие логи

			req := httptest.NewRequest(http.MethodGet, uri, nil)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if logs.Len() != 1 {
				t.Fatalf("Expected 1 log entry, got %d", logs.Len())
			}

			fields := logs.All()[0].ContextMap()

			if fields["uri"] != uri {
				t.Errorf("Expected uri %s, got %v", uri, fields["uri"])
			}
		})
	}
}

func TestLoggingResponseWriter_WriteHeaderOnce(t *testing.T) {
	w := httptest.NewRecorder()
	lrw := &loggingResponseWriter{
		ResponseWriter: w,
	}

	// Первый вызов
	lrw.WriteHeader(http.StatusOK)
	if lrw.status != http.StatusOK {
		t.Errorf("Expected status 200, got %d", lrw.status)
	}

	// Второй вызов не должен изменить статус
	lrw.WriteHeader(http.StatusBadRequest)
	if lrw.status != http.StatusOK {
		t.Errorf("Expected status to remain 200, got %d", lrw.status)
	}
}

func TestLoggingResponseWriter_WriteImplicitHeader(t *testing.T) {
	w := httptest.NewRecorder()
	lrw := &loggingResponseWriter{
		ResponseWriter: w,
	}

	// Write без явного WriteHeader должен установить 200
	lrw.Write([]byte("test"))

	if lrw.status != http.StatusOK {
		t.Errorf("Expected implicit status 200, got %d", lrw.status)
	}

	if !lrw.wroteHeader {
		t.Error("Expected wroteHeader to be true")
	}

	if lrw.size != 4 {
		t.Errorf("Expected size 4, got %d", lrw.size)
	}
}

func TestZapLoggerMiddleware_Duration(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ZapLoggerMiddleware(logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if logs.Len() != 1 {
		t.Fatalf("Expected 1 log entry, got %d", logs.Len())
	}

	fields := logs.All()[0].ContextMap()

	// Проверяем, что duration существует и имеет правильный тип
	if duration, ok := fields["duration"]; !ok {
		t.Error("Expected duration field to be present")
	} else {
		// duration хранится как time.Duration
		if _, ok := duration.(time.Duration); !ok {
			t.Errorf("Expected duration to be time.Duration, got %T", duration)
		}
		// Не проверяем конкретное значение, так как на быстрых машинах может быть 0
	}
}

func TestZapLoggerMiddleware_LogMessage(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ZapLoggerMiddleware(logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if logs.Len() != 1 {
		t.Fatalf("Expected 1 log entry, got %d", logs.Len())
	}

	logEntry := logs.All()[0]

	// Проверяем сообщение лога
	if logEntry.Message != "HTTP request" {
		t.Errorf("Expected message 'HTTP request', got %s", logEntry.Message)
	}

	// Проверяем уровень лога
	if logEntry.Level != zap.InfoLevel {
		t.Errorf("Expected Info level, got %v", logEntry.Level)
	}
}
