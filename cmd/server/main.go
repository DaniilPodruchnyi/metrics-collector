package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/server"
)

func main() {
	// Парсим конфигурацию
	cfg, err := config.ParseServerConfig()
	if err != nil {
		log.Fatalf("Failed to parse configuration: %v", err)
	}

	// Логируем конфигурацию
	cfg.LogConfig()

	// Инициализация сервера
	srv := server.New(cfg)

	// Обработка graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		srv.Stop()
		os.Exit(0)
	}()

	log.Printf("Server starting on %s", cfg.Address)

	// Запуск сервера
	if err := srv.Start(); err != nil {
		log.Fatal("Server failed:", err)
	}
}
