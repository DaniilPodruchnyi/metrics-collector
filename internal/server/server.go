package server

import (
	"log"
	"net/http"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/handler"
	custommiddleware "github.com/DaniilPodruchnyi/metrics-collector/internal/middleware"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// Структура сервера по работе с метриками
type Server struct {
	address  string
	service  *service.MetricService
	handlers *handler.MetricHandler
}

// Функция для инициализации сервера
func New(address string) *Server {
	metricsRepository := repository.New()
	metricService := service.New(metricsRepository)
	metricHandler := handler.New(metricService)

	return &Server{
		address:  address,
		service:  metricService,
		handlers: metricHandler,
	}
}

// Метод для запуска сервера
func (s *Server) Start() error {
	router := s.setupRoutes()

	log.Printf("Server listening on %s", s.address)
	return http.ListenAndServe(s.address, router)
}

// Mock-метод для остановки сервера
func (s *Server) Stop() {
	log.Println("Server is stopped")
}

// Метод для настройки роутера сервера
func (s *Server) setupRoutes() chi.Router {
	// Используем роутер chi
	r := chi.NewRouter()
	// Мой кастомный логгер
	logger, _ := zap.NewProduction()

	// Добавляем стандартные middleware
	r.Use(middleware.StripSlashes)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// Добавляем gzip middleware
	r.Use(custommiddleware.GzipMiddleware)

	// Подключаем zap-логирование
	r.Use(custommiddleware.ZapLoggerMiddleware(logger))

	r.Post("/update", s.handlers.UpdateMetricsJSON)
	r.Post("/value", s.handlers.GetMetricJSON)

	// Маршруты для обновления метрик
	r.Post("/update/{type}/{name}/{value}", s.handlers.UpdateMetrics)

	// Маршруты для получения метрик
	r.Get("/value/{type}/{name}", s.handlers.GetMetricValue)
	r.Get("/", s.handlers.GetAllMetricsHTML)

	return r
}
