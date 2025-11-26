package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/config/db"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/handler"
	custommiddleware "github.com/DaniilPodruchnyi/metrics-collector/internal/middleware"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Структура сервера по работе с метриками
type Server struct {
	address     string
	config      *config.ServerConfig
	service     *service.MetricService
	handlers    *handler.MetricHandler
	fileStorage storage.PersistentStorage
	repository  repository.MetricRepository
	httpServer  *http.Server
	cancelFunc  context.CancelFunc
	dbPool      *pgxpool.Pool
}

// Функция для инициализации сервера
func New(cfg *config.ServerConfig) *Server {
	// Создаем файловое хранилище (через интерфейс)
	var fileStorage storage.PersistentStorage = storage.NewFileStorage(cfg.FileStoragePath)

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

	// Инициализируем БД, если DSN указан
	var pool *pgxpool.Pool
	if cfg.DatabaseDSN != "" {
		dbConfig := db.DefaultPostgresConfig(cfg.DatabaseDSN)
		var err error
		pool, err = db.NewPostgresPool(context.Background(), dbConfig)
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v", err)
			log.Println("Server will continue without database")
		} else {
			log.Println("Successfully connected to PostgreSQL")
		}
	}

	metricService := service.New(metricsRepository)
	metricHandler := handler.New(metricService, pool)

	return &Server{
		address:     cfg.Address,
		config:      cfg,
		service:     metricService,
		handlers:    metricHandler,
		fileStorage: fileStorage,
		repository:  metricsRepository,
		dbPool:      pool,
	}
}

// Метод для запуска сервера
func (s *Server) Start(ctx context.Context) error {
	router := s.setupRoutes()

	// Создаем контекст с отменой для graceful shutdown
	serverCtx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	// Запускаем периодическое сохранение, если интервал > 0
	if !s.config.IsSyncMode() {
		go s.startPeriodicSave(serverCtx)
	}

	s.httpServer = &http.Server{
		Addr:    s.address,
		Handler: router,
	}

	log.Printf("Server listening on %s", s.address)

	// Запускаем сервер
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
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

// Shutdown выполняет graceful shutdown
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Server is shutting down...")

	// Отменяем контекст для остановки периодического сохранения
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	// Закрываем пул соединений
	if s.dbPool != nil {
		log.Println("Closing database connection pool...")
		s.dbPool.Close()
	}

	// Сохраняем метрики перед выходом
	log.Println("Saving metrics before shutdown...")
	if err := s.saveMetrics(); err != nil {
		log.Printf("Failed to save metrics on shutdown: %v", err)
	} else {
		log.Printf("Metrics saved successfully on shutdown")
	}

	// Graceful shutdown HTTP сервера
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}

	return nil
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

	r.Get("/ping", s.handlers.PingDB)
	// Роуты с проверкой статуса
	r.Post("/update", s.wrapWithSyncSmart(s.handlers.UpdateMetricsJSON))
	r.Post("/value", s.handlers.GetMetricJSON)
	r.Post("/update/{type}/{name}/{value}", s.wrapWithSyncSmart(s.handlers.UpdateMetrics))
	r.Get("/value/{type}/{name}", s.handlers.GetMetricValue)
	r.Get("/", s.handlers.GetAllMetricsHTML)

	return r
}

// responseRecorder перехватывает статус ответа
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rr *responseRecorder) WriteHeader(code int) {
	if !rr.written {
		rr.statusCode = code
		rr.written = true
		rr.ResponseWriter.WriteHeader(code)
	}
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	if !rr.written {
		rr.WriteHeader(http.StatusOK)
	}
	return rr.ResponseWriter.Write(b)
}

// wrapWithSyncSmart оборачивает handler для синхронного сохранения только при успехе
func (s *Server) wrapWithSyncSmart(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		handler(recorder, r)

		// Сохраняем только если запрос успешен (2xx)
		if recorder.statusCode >= 200 && recorder.statusCode < 300 {
			s.SaveOnUpdate()
		}
	}
}
