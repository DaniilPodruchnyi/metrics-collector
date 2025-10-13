package main

import (
	"log"

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

	// Создаем и запускаем агента
	agentInstance := agent.New(cfg)
	agentInstance.Run()
}
