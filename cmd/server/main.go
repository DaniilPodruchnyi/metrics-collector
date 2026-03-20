package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/buildinfo"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/server"
)

var buildVersion string
var buildDate string
var buildCommit string

func main() {
	buildinfo.Print(os.Stdout, buildinfo.Info{
		Version: buildVersion,
		Date:    buildDate,
		Commit:  buildCommit,
	})

	// Парсим конфигурацию.
	// Поддерживаем:
	// - JSON-конфиг файла: `-c <path>` или `-config <path>`
	// - Асимметричное шифрование (RSA):
	//   `-crypto-key <path>` — путь к приватному ключу для режима шифрования
	cfg, err := config.ParseServerConfig()
	if err != nil {
		log.Fatalf("Failed to parse configuration: %v", err)
	}

	// Логируем конфигурацию
	cfg.LogConfig()

	// Инициализация сервера
	srv := server.New(cfg)

	// Создаем контекст для управления жизненным циклом
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Канал для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	// os.Interrupt == SIGINT, не дублируем явное syscall.SIGINT
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	// Запускаем сервер в горутине
	go func() {
		log.Printf("Starting server on %s", cfg.Address)
		if err := srv.Start(ctx); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Ждем сигнал остановки
	sig := <-sigChan
	log.Printf("Received signal: %v", sig)

	// Создаем контекст с таймаутом для graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Выполняем graceful shutdown
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped gracefully")
}
