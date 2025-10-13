package server

import (
	"log"
	"net/http"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/handler"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
func (s *Server) setupRoutes() chi.Router {
	// Используем роутер chi
	r := chi.NewRouter()

	// Добавляем стандартные middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// Маршруты для обновления метрик
	r.Post("/update/{type}/{name}/{value}", s.handlers.UpdateMetrics)

	// Маршруты для получения метрик
	r.Get("/value/{type}/{name}", s.handlers.GetMetricValue)
	r.Get("/", s.handlers.GetAllMetricsHTML)

	return r
}
