package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/handler"
	custommiddleware "github.com/DaniilPodruchnyi/metrics-collector/internal/middleware"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// Структура сервера по работе с метриками
type Server struct {
	address     string
	config      *config.ServerConfig
	service     *service.MetricService
	handlers    *handler.MetricHandler
	fileStorage *storage.FileStorage
	repository  *repository.MemStorage
}

// Функция для инициализации сервера
func New(cfg *config.ServerConfig) *Server {
	// Создаем файловое хранилище
	fileStorage := storage.NewFileStorage(cfg.FileStoragePath)

	// Создаем repository
	var metricsRepository *repository.MemStorage

	// Загружаем данные из файла, если нужно
	if cfg.Restore {
		log.Printf("Restoring metrics from %s", cfg.FileStoragePath)
		data, err := fileStorage.Load()
		if err != nil {
			log.Printf("Failed to load metrics: %v. Starting with empty storage", err)
			metricsRepository = repository.New()
		} else {
			log.Printf("Successfully restored %d metrics", len(data))
			metricsRepository = repository.NewWithData(data)
		}
	} else {
		metricsRepository = repository.New()
	}

	metricService := service.New(metricsRepository)
	metricHandler := handler.New(metricService)

	return &Server{
		address:     cfg.Address,
		config:      cfg,
		service:     metricService,
		handlers:    metricHandler,
		fileStorage: fileStorage,
		repository:  metricsRepository,
	}
}

// Метод для запуска сервера
func (s *Server) Start() error {
	router := s.setupRoutes()

	// Запускаем периодическое сохранение, если интервал > 0
	if !s.config.IsSyncMode() {
		go s.startPeriodicSave(context.Background())
	}

	log.Printf("Server listening on %s", s.address)
	return http.ListenAndServe(s.address, router)
}

// startPeriodicSave запускает периодическое сохранение метрик
func (s *Server) startPeriodicSave(ctx context.Context) {
	ticker := time.NewTicker(s.config.StoreInterval)
	defer ticker.Stop()

	log.Printf("Starting periodic save with interval: %v", s.config.StoreInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping periodic save")
			return
		case <-ticker.C:
			if err := s.saveMetrics(); err != nil {
				log.Printf("Failed to save metrics: %v", err)
			} else {
				log.Printf("Successfully saved metrics to %s", s.config.FileStoragePath)
			}
		}
	}
}

// saveMetrics сохраняет текущие метрики в файл
func (s *Server) saveMetrics() error {
	metrics := s.service.GetAllMetrics()
	return s.fileStorage.Save(metrics)
}

// SaveOnUpdate сохраняет метрики синхронно после каждого обновления
func (s *Server) SaveOnUpdate() {
	if s.config.IsSyncMode() {
		if err := s.saveMetrics(); err != nil {
			log.Printf("Failed to save metrics on update: %v", err)
		}
	}
}

// Метод для остановки сервера
func (s *Server) Stop() {
	log.Println("Server is stopping, saving metrics...")
	if err := s.saveMetrics(); err != nil {
		log.Printf("Failed to save metrics on shutdown: %v", err)
	} else {
		log.Printf("Metrics saved successfully on shutdown")
	}
}

// Метод для настройки роутера сервера
func (s *Server) setupRoutes() chi.Router {
	r := chi.NewRouter()
	logger, _ := zap.NewProduction()

	// ВАЖНО: StripSlashes должен быть первым
	r.Use(middleware.StripSlashes)

	// Добавляем gzip middleware
	r.Use(custommiddleware.GzipMiddleware)

	// Остальные middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(custommiddleware.ZapLoggerMiddleware(logger))

	// Роуты
	r.Post("/update", s.wrapWithSync(s.handlers.UpdateMetricsJSON))
	r.Post("/value", s.handlers.GetMetricJSON)
	r.Post("/update/{type}/{name}/{value}", s.wrapWithSync(s.handlers.UpdateMetrics))
	r.Get("/value/{type}/{name}", s.handlers.GetMetricValue)
	r.Get("/", s.handlers.GetAllMetricsHTML)

	return r
}

// wrapWithSync оборачивает handler для синхронного сохранения
func (s *Server) wrapWithSync(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
		s.SaveOnUpdate()
	}
}
