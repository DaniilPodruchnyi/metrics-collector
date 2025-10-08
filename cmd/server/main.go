package main

import (
	"log"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/server"
)

func main() {
	// Инициализация сервера
	srv := server.New()

	log.Println("Server starting on :8080")

	// Запуск сервера
	if err := srv.Start(); err != nil {
		log.Fatal("Server failed:", err)
	}
}
