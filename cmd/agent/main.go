package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/agent"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
)

func main() {
	// Парсим конфигурацию
	cfg, err := config.ParseAgentConfig()
	if err != nil {
		log.Fatalf("Failed to parse configuration: %v", err)
	}

	// Логируем конфигурацию
	cfg.LogConfig()

	// Создаем агента
	agentInstance := agent.New(cfg)

	// Создаем контекст с отменой для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Канал для приема OS сигналов
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Запускаем агента в отдельной горутине
	done := make(chan struct{})
	go func() {
		agentInstance.Run(ctx)
		close(done)
	}()

	// Ждем сигнал завершения
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)

		// Отменяем контекст для всех горутин
		cancel()

		// Ждем завершения агента с таймаутом
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := agentInstance.Shutdown(shutdownCtx); err != nil {
			log.Printf("Shutdown error: %v", err)
			os.Exit(1)
		}

	case <-done:
		log.Println("Agent stopped normally")
	}

	log.Println("Agent exited gracefully")
}
