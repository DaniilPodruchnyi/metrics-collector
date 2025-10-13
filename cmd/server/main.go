package main

import (
	"log"

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
	srv := server.New(cfg.Address)

	log.Printf("Server starting on %s", cfg.Address)

	// Запуск сервера
	if err := srv.Start(); err != nil {
		log.Fatal("Server failed:", err)
	}
}
