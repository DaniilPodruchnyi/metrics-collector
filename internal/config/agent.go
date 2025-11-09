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
}

func ParseAgentConfig() (*AgentConfig, error) {
	// Значения по умолчанию
	const (
		defaultServerAddr = "localhost:8080"
		defaultPoll       = 2
		defaultReport     = 10
	)
	var (
		serverAddrFlag = flag.String("a", "", "server address")
		pollFlag       = flag.Int("p", defaultPoll, "poll interval in seconds")
		reportFlag     = flag.Int("r", defaultReport, "report interval in seconds")
	)
	flag.Parse()

	// 1. ADDRESS
	serverAddr := getEnvOrFlagString("ADDRESS", *serverAddrFlag, defaultServerAddr)
	// 2. POLL_INTERVAL
	pollInterval := getEnvOrFlagInt("POLL_INTERVAL", *pollFlag, defaultPoll)
	// 3. REPORT_INTERVAL
	reportInterval := getEnvOrFlagInt("REPORT_INTERVAL", *reportFlag, defaultReport)

	config := &AgentConfig{
		ServerAddress:  normalizeServerAddress(serverAddr),
		PollInterval:   time.Duration(pollInterval) * time.Second,
		ReportInterval: time.Duration(reportInterval) * time.Second,
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
