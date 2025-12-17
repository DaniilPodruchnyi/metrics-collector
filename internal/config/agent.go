package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// AgentConfig содержит конфигурацию агента
type AgentConfig struct {
	ServerAddress  string
	PollInterval   time.Duration
	ReportInterval time.Duration
	Key            string // Ключ для подписи запросов
	RateLimit      int    // Максимальное количество одновременных запросов
}

func ParseAgentConfig() (*AgentConfig, error) {
	// Значения по умолчанию
	const (
		defaultServerAddr = "localhost:8080"
		defaultPoll       = 2
		defaultReport     = 10
		defaultRateLimit  = 3 // По умолчанию 3 одновременных запроса
	)
	var (
		serverAddrFlag = flag.String("a", "", "server address")
		pollFlag       = flag.Int("p", defaultPoll, "poll interval in seconds")
		reportFlag     = flag.Int("r", defaultReport, "report interval in seconds")
		keyFlag        = flag.String("k", "", "key for signing requests (SHA256)")
		rateLimitFlag  = flag.Int("l", defaultRateLimit, "rate limit (max concurrent requests)")
	)
	flag.Parse()

	// 1. ADDRESS
	serverAddr := getEnvOrFlagString("ADDRESS", *serverAddrFlag, defaultServerAddr)
	// 2. POLL_INTERVAL
	pollInterval := getEnvOrFlagInt("POLL_INTERVAL", *pollFlag, defaultPoll)
	// 3. REPORT_INTERVAL
	reportInterval := getEnvOrFlagInt("REPORT_INTERVAL", *reportFlag, defaultReport)
	// 4. KEY
	key := getEnvOrFlagString("KEY", *keyFlag, "")
	// 5. RATE_LIMIT
	rateLimit := getEnvOrFlagInt("RATE_LIMIT", *rateLimitFlag, defaultRateLimit)

	config := &AgentConfig{
		ServerAddress:  normalizeServerAddress(serverAddr),
		PollInterval:   time.Duration(pollInterval) * time.Second,
		ReportInterval: time.Duration(reportInterval) * time.Second,
		Key:            key,
		RateLimit:      rateLimit,
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return config, nil
}

func getEnvOrFlagString(envKey string, flagVal string, defaultVal string) string {
	if envVal := os.Getenv(envKey); envVal != "" {
		return envVal
	}
	if flagVal != "" {
		return flagVal
	}
	return defaultVal
}

func getEnvOrFlagInt(envKey string, flagVal int, defaultVal int) int {
	if envVal := os.Getenv(envKey); envVal != "" {
		if parsed, err := strconv.Atoi(envVal); err == nil {
			return parsed
		}
	}
	// если флаг не дефолт, то используем флаг (иначе - env, иначе дефолт)
	if flagVal != defaultVal {
		return flagVal
	}
	return defaultVal
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

	if c.RateLimit <= 0 {
		return fmt.Errorf("rate limit must be positive, got %d", c.RateLimit)
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
	keyInfo := "none"
	if c.Key != "" {
		keyInfo = "configured"
	}
	return fmt.Sprintf("Agent{Server: %s, Poll: %v, Report: %v, Key: %s, RateLimit: %d}",
		c.ServerAddress, c.PollInterval, c.ReportInterval, keyInfo, c.RateLimit)
}

// LogConfig выводит конфигурацию в лог
func (c *AgentConfig) LogConfig() {
	log.Printf("Agent configuration: %s", c.String())
}

// GetServerURL возвращает URL сервера для метрик
func (c *AgentConfig) GetServerURL() string {
	return c.ServerAddress
}

// HasKey возвращает true, если ключ для подписи установлен
func (c *AgentConfig) HasKey() bool {
	return c.Key != ""
}
