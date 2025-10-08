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
	handlers *handler.Handlers
}

// Функция для инициализации сервера
func New() *Server {
	metricsRepository := repository.New()
	metricService := service.New(*metricsRepository)
	handler := handler.New(*metricService)

	return &Server{
		service:  metricService,
		handlers: handler,
	}
}

// Метод для запуска сервера
func (s *Server) Start() error {
	router := s.setupRoutes()

	log.Println("Server listinig on :8080")
	return http.ListenAndServe(":8080", router)
}

// Mock-метод для  остановки сервера
func (s *Server) Stop() {
	log.Println("Server is stopped")
}

// Метод для настройки роутера сервера
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/update/{type}/{name}/{value}", s.handlers.UpdateMetrics)

	return mux
}
