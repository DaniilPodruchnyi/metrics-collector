package config

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"
)

// AgentConfig содержит конфигурацию агента
type AgentConfig struct {
	ServerAddress  string
	PollInterval   time.Duration
	ReportInterval time.Duration
}

// ParseAgentConfig парсит флаги командной строки для агента
func ParseAgentConfig() (*AgentConfig, error) {
	config := &AgentConfig{}

	// Временные переменные для парсинга интервалов в секундах
	var (
		serverAddr     string
		pollInterval   int
		reportInterval int
	)

	// Определяем флаги с значениями по умолчанию
	flag.StringVar(&serverAddr, "a", "localhost:8080", "server address")
	flag.IntVar(&pollInterval, "p", 2, "poll interval in seconds")
	flag.IntVar(&reportInterval, "r", 10, "report interval in seconds")

	// Парсим флаги
	flag.Parse()

	// Проверяем, что нет неизвестных аргументов
	if flag.NArg() > 0 {
		return nil, fmt.Errorf("unknown arguments: %v", flag.Args())
	}

	// Заполняем конфигурацию
	config.ServerAddress = normalizeServerAddress(serverAddr)
	config.PollInterval = time.Duration(pollInterval) * time.Second
	config.ReportInterval = time.Duration(reportInterval) * time.Second

	// Валидируем конфигурацию
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return config, nil
}

// validate проверяет корректность конфигурации агента
func (c *AgentConfig) validate() error {
	if c.ServerAddress == "" {
		return fmt.Errorf("server address cannot be empty")
	}

	if c.PollInterval <= 0 {
		return fmt.Errorf("poll interval must be positive, got %v", c.PollInterval)
	}

	if c.ReportInterval <= 0 {
		return fmt.Errorf("report interval must be positive, got %v", c.ReportInterval)
	}

	if c.PollInterval >= c.ReportInterval {
		log.Printf("Warning: poll interval (%v) should be less than report interval (%v)",
			c.PollInterval, c.ReportInterval)
	}

	return nil
}

// normalizeServerAddress добавляет http:// если отсутствует
func normalizeServerAddress(addr string) string {
	if addr == "" {
		return ""
	}

	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		return "http://" + addr
	}
	return addr
}

// String возвращает строковое представление конфигурации
func (c *AgentConfig) String() string {
	return fmt.Sprintf("Agent{Server: %s, Poll: %v, Report: %v}",
		c.ServerAddress, c.PollInterval, c.ReportInterval)
}

// LogConfig выводит конфигурацию в лог
func (c *AgentConfig) LogConfig() {
	log.Printf("Agent configuration: %s", c.String())
}

// GetServerURL возвращает URL сервера для метрик
func (c *AgentConfig) GetServerURL() string {
	return c.ServerAddress
}
