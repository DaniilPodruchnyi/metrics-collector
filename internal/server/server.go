package server

import (
	"log"
	"net/http"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/handler"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
)

// Структура сервера по работе с метриками
type Server struct {
	service  *service.MetricService
	handlers *handler.MetricHandler
}

// Функция для инициализации сервера
func New() *Server {
	metricsRepository := repository.New()
	metricService := service.New(metricsRepository)
	metricHandler := handler.New(metricService)

	return &Server{
		service:  metricService,
		handlers: metricHandler,
	}
}

// Метод для запуска сервера
func (s *Server) Start() error {
	router := s.setupRoutes()

	log.Println("Server listening on :8080")
	return http.ListenAndServe(":8080", router)
}

// Mock-метод для остановки сервера
func (s *Server) Stop() {
	log.Println("Server is stopped")
}

// Метод для настройки роутера сервера
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Основные роуты с точными паттернами
	mux.HandleFunc("POST /update/{type}/{name}/{value}", s.handlers.UpdateMetrics)
	mux.HandleFunc("GET /value/{name}", s.handlers.GetMetric)
	mux.HandleFunc("GET /{$}", s.handlers.GetAllMetrics) // Только корень

	// Обработчики для неполных путей - возвращают 404
	mux.HandleFunc("/update/{type}/{name}/", s.handle404) // /update/counter/name/
	mux.HandleFunc("/update/{type}/", s.handle404)        // /update/counter/
	mux.HandleFunc("/update/", s.handle404)               // /update/
	mux.HandleFunc("/value/", s.handle404)                // /value/

	// для всех остальных путей - возвращает 404
	mux.HandleFunc("/", s.handleCatchAll)

	return mux
}

// handle404 отправляет 404 ответ для неполных путей
func (s *Server) handle404(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// handleCatchAll обрабатывает все остальные пути
func (s *Server) handleCatchAll(w http.ResponseWriter, r *http.Request) {
	// Если это не корневой путь, возвращаем 404
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Для корневого пути вызываем GetAllMetrics только для GET запросов
	if r.Method == http.MethodGet {
		s.handlers.GetAllMetrics(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
