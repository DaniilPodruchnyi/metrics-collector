package server

import (
	"context"
	"crypto/rsa"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/config/db"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/handler"
	custommiddleware "github.com/DaniilPodruchnyi/metrics-collector/internal/middleware"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	metrics "github.com/DaniilPodruchnyi/metrics-collector/internal/proto"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/repository"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/security"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/service"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server инкапсулирует HTTP-сервер, хранилище и обработчики метрик.
type Server struct {
	metrics.UnimplementedMetricsServer

	address     string
	config      *config.ServerConfig
	service     *service.MetricService
	handlers    *handler.MetricHandler
	fileStorage storage.PersistentStorage
	repository  repository.MetricRepository
	httpServer  *http.Server
	cancelFunc  context.CancelFunc
	dbPool      *pgxpool.Pool
	storageType string // "postgres", "file", или "memory"
	privateKey  *rsa.PrivateKey
	trustedCIDR *net.IPNet
	grpcServer  *grpc.Server
}

// New создает новый сервер метрик на основе конфигурации.
func New(cfg *config.ServerConfig) *Server {
	var metricsRepository repository.MetricRepository
	var fileStorage storage.PersistentStorage
	var pool *pgxpool.Pool
	var storageType string

	// Приоритет 1: PostgreSQL (если DSN указан)
	if cfg.DatabaseDSN != "" {
		log.Println("Attempting to connect to PostgreSQL...")
		dbConfig := db.DefaultPostgresConfig(cfg.DatabaseDSN)
		var err error
		pool, err = db.NewPostgresPool(context.Background(), dbConfig)
		if err != nil {
			log.Printf("Warning: Failed to connect to PostgreSQL: %v", err)
			log.Println("Falling back to file/memory storage")
		} else {
			// Запускаем миграции
			if err := db.RunMigrations(context.Background(), pool, "migrations"); err != nil {
				log.Printf("Warning: Migration failed: %v", err)
				pool.Close()
				pool = nil
			} else {
				log.Println("Successfully connected to PostgreSQL and ran migrations")
				metricsRepository = repository.NewPostgresRepository(pool)
				storageType = "postgres"
			}
		}
	}

	// Приоритет 2: File storage (если PostgreSQL не доступен и путь указан)
	if metricsRepository == nil && cfg.FileStoragePath != "" {
		log.Printf("Using file storage: %s", cfg.FileStoragePath)
		fileStorage = storage.NewFileStorage(cfg.FileStoragePath)
		storageType = "file"

		// Загружаем данные из файла при Restore
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
	}

	// Приоритет 3: In-memory storage
	if metricsRepository == nil {
		log.Println("Using in-memory storage")
		metricsRepository = repository.New()
		storageType = "memory"
	}

	metricService := service.New(metricsRepository)
	metricHandler := handler.New(metricService, pool)

	log.Printf("Storage type selected: %s", storageType)

	s := &Server{
		address:     cfg.Address,
		config:      cfg,
		service:     metricService,
		handlers:    metricHandler,
		fileStorage: fileStorage,
		repository:  metricsRepository,
		dbPool:      pool,
		storageType: storageType,
	}

	// Загружаем приватный ключ для асимметричного шифрования, если указан путь
	if cfg.HasCryptoKeyPath() {
		priv, err := security.LoadPrivateKeyFromFile(cfg.CryptoKeyPath)
		if err != nil {
			log.Printf("Failed to load private key from %s: %v. Asymmetric decryption disabled.", cfg.CryptoKeyPath, err)
		} else {
			s.privateKey = priv
			log.Printf("Asymmetric decryption enabled with private key: %s", cfg.CryptoKeyPath)
		}
	}

	// Парсим доверенную подсеть, если она указана
	if cfg.TrustedSubnet != "" {
		if _, ipnet, err := net.ParseCIDR(cfg.TrustedSubnet); err != nil {
			log.Printf("Invalid trusted subnet %q: %v. Trusted subnet checks disabled.", cfg.TrustedSubnet, err)
		} else {
			s.trustedCIDR = ipnet
			log.Printf("Trusted subnet configured: %s", cfg.TrustedSubnet)
		}
	}

	return s
}

// UpdateMetrics реализует gRPC-сервис Metrics.UpdateMetrics.
// Использует batch‑обновление метрик и ту же бизнес-логику, что и HTTP‑обработчики.
func (s *Server) UpdateMetrics(ctx context.Context, req *metrics.UpdateMetricsRequest) (*metrics.UpdateMetricsResponse, error) {
	if req == nil || len(req.Metrics) == 0 {
		return &metrics.UpdateMetricsResponse{}, nil
	}

	batch := make([]model.Metrics, 0, len(req.Metrics))
	for _, m := range req.Metrics {
		if m == nil {
			continue
		}

		var (
			mType string
			delta *int64
			value *float64
		)

		switch m.GetType() {
		case metrics.Metric_COUNTER:
			mType = model.Counter
			d := m.GetDelta()
			delta = &d
		case metrics.Metric_GAUGE:
			mType = model.Gauge
			v := m.GetValue()
			value = &v
		default:
			log.Printf("skipping metric %q: unknown gRPC metric type %v (%d)",
				m.GetId(), m.GetType(), int32(m.GetType()))
			continue
		}

		batch = append(batch, model.Metrics{
			ID:    m.GetId(),
			MType: mType,
			Delta: delta,
			Value: value,
		})
	}

	if err := s.service.UpdateMetricsBatch(batch); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update metrics batch: %v", err)
	}

	// Сохраняем метрики, если настроен синхронный режим для файлового хранилища.
	s.SaveOnUpdate()

	return &metrics.UpdateMetricsResponse{}, nil
}

// Start запускает HTTP-сервер и, при необходимости, фоновое сохранение метрик.
func (s *Server) Start(ctx context.Context) error {
	router := s.setupRoutes()

	serverCtx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	// При необходимости запускаем gRPC‑сервер в отдельной горутине.
	if s.config.GRPCAddress != "" {
		go s.startGRPCServer(serverCtx)
	}

	// Запускаем периодическое сохранение ТОЛЬКО для file storage
	if s.storageType == "file" && !s.config.IsSyncMode() {
		go s.startPeriodicSave(serverCtx)
	}

	s.httpServer = &http.Server{
		Addr:    s.address,
		Handler: router,
	}

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

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

func (s *Server) saveMetrics() error {
	// Сохраняем только для file storage
	if s.storageType != "file" || s.fileStorage == nil {
		return nil
	}

	metrics := s.service.GetAllMetrics()
	return s.fileStorage.Save(metrics)
}

// SaveOnUpdate выполняет сохранение метрик сразу после успешного обновления.
func (s *Server) SaveOnUpdate() {
	// Сохраняем только для file storage в синхронном режиме
	if s.storageType == "file" && s.config.IsSyncMode() {
		if err := s.saveMetrics(); err != nil {
			log.Printf("Failed to save metrics on update: %v", err)
		}
	}
	// Для PostgreSQL ничего не делаем - данные уже в БД
}

// Shutdown корректно останавливает сервер и освобождает ресурсы.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Server is shutting down...")

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	// Останавливаем gRPC‑сервер, если он запущен.
	if s.grpcServer != nil {
		log.Println("Stopping gRPC server...")
		stopped := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(stopped)
		}()

		select {
		case <-stopped:
		case <-ctx.Done():
			// Если graceful‑остановка не удалась за время таймаута — принудительно останавливаем.
			s.grpcServer.Stop()
		}
	}

	// Закрываем пул соединений
	if s.dbPool != nil {
		log.Println("Closing database connection pool...")
		s.dbPool.Close()
	}

	// Сохраняем метрики для file storage перед выходом
	if s.storageType == "file" {
		log.Println("Saving metrics before shutdown...")
		if err := s.saveMetrics(); err != nil {
			log.Printf("Failed to save metrics on shutdown: %v", err)
		} else {
			log.Printf("Metrics saved successfully on shutdown")
		}
	}

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}

// setupRoutes настраивает HTTP-маршруты и middleware сервера.
func (s *Server) setupRoutes() chi.Router {
	r := chi.NewRouter()
	logger, _ := zap.NewProduction()

	r.Use(middleware.StripSlashes)
	// ВАЖНО: GzipMiddleware должен быть ПЕРЕД CryptoMiddleware и HashVerificationMiddleware
	r.Use(custommiddleware.GzipMiddleware)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(custommiddleware.ZapLoggerMiddleware(logger))

	// Расшифровка запросов (если настроен приватный ключ)
	if s.privateKey != nil {
		log.Println("Asymmetric decryption middleware enabled")
		r.Use(custommiddleware.CryptoMiddleware(s.privateKey))
	}

	// Hash middleware - применяются после gzip декомпрессии
	if s.config.HasKey() {
		log.Println("Hash verification and signing enabled")
		r.Use(custommiddleware.HashVerificationMiddleware(s.config.Key))
		r.Use(custommiddleware.HashSigningMiddleware(s.config.Key))
	}

	// Batch endpoint - добавляем оба варианта (с и без slash)
	r.Post("/updates", s.wrapWithSyncSmart(s.wrapWithTrustedSubnetCheck(s.handlers.UpdateMetricsBatch)))
	r.Post("/updates/", s.wrapWithSyncSmart(s.wrapWithTrustedSubnetCheck(s.handlers.UpdateMetricsBatch)))

	r.Post("/update", s.wrapWithSyncSmart(s.wrapWithTrustedSubnetCheck(s.handlers.UpdateMetricsJSON)))
	r.Post("/value", s.handlers.GetMetricJSON)
	r.Post("/update/{type}/{name}/{value}", s.wrapWithSyncSmart(s.wrapWithTrustedSubnetCheck(s.handlers.UpdateMetrics)))
	r.Get("/value/{type}/{name}", s.handlers.GetMetricValue)
	r.Get("/", s.handlers.GetAllMetricsHTML)
	r.Get("/ping", s.handlers.PingDB)

	return r
}

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

func (s *Server) wrapWithSyncSmart(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		handler(recorder, r)

		if recorder.statusCode >= 200 && recorder.statusCode < 300 {
			s.SaveOnUpdate()
		}
	}
}

// wrapWithTrustedSubnetCheck добавляет проверку X-Real-IP против доверенной подсети.
// Если доверенная подсеть не настроена, запросы пропускаются без ограничений.
func (s *Server) wrapWithTrustedSubnetCheck(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Если trusted subnet не настроена — пропускаем без проверок
		if s.trustedCIDR == nil {
			next(w, r)
			return
		}

		ipStr := r.Header.Get("X-Real-IP")
		if ipStr == "" {
			http.Error(w, "missing X-Real-IP header", http.StatusForbidden)
			return
		}

		ip := net.ParseIP(ipStr)
		if ip == nil {
			http.Error(w, "invalid X-Real-IP header", http.StatusForbidden)
			return
		}

		if !s.trustedCIDR.Contains(ip) {
			http.Error(w, "forbidden: IP not in trusted subnet", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
