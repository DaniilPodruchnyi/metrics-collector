package config

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"encoding/json"
)

// AgentConfig содержит конфигурацию агента
type AgentConfig struct {
	ServerAddress  string
	PollInterval   time.Duration
	ReportInterval time.Duration
	Key            string // Ключ для подписи запросов
	RateLimit      int    // Максимальное количество одновременных запросов
	CryptoKeyPath  string // Путь к файлу с публичным ключом (RSA)
}

// agentConfigFile описывает формат JSON-конфигурации агента
type agentConfigFile struct {
	Address        string `json:"address"`
	ReportInterval string `json:"report_interval"`
	PollInterval   string `json:"poll_interval"`
	Key            string `json:"key"`
	RateLimit      *int   `json:"rate_limit"`
	CryptoKey      string `json:"crypto_key"`
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
		configPath    string
		serverAddrFlag = flag.String("a", "", "server address")
		pollFlag       = flag.Int("p", defaultPoll, "poll interval in seconds")
		reportFlag     = flag.Int("r", defaultReport, "report interval in seconds")
		keyFlag        = flag.String("k", "", "key for signing requests (SHA256)")
		rateLimitFlag  = flag.Int("l", defaultRateLimit, "rate limit (max concurrent requests)")
		cryptoKeyFlag  = flag.String("crypto-key", "", "path to public key file for asymmetric encryption (RSA)")
	)

	// Путь к файлу конфигурации: флаги -c / -config
	flag.StringVar(&configPath, "c", "", "path to configuration file (JSON)")
	flag.StringVar(&configPath, "config", "", "path to configuration file (JSON)")

	flag.Parse()

	// Если путь не задан флагом, пробуем переменную окружения CONFIG
	if configPath == "" {
		configPath = os.Getenv("CONFIG")
	}

	var fileCfg agentConfigFile
	if configPath != "" {
		cfgFromFile, err := loadAgentConfigFromFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent config file: %w", err)
		}
		if cfgFromFile != nil {
			fileCfg = *cfgFromFile
		}
	}

	// Приоритет опций: env -> flag -> config file -> default

	// ADDRESS
	serverAddr := defaultServerAddr
	if fileCfg.Address != "" {
		serverAddr = fileCfg.Address
	}
	if env := os.Getenv("ADDRESS"); env != "" {
		serverAddr = env
	} else if f := flag.Lookup("a"); f != nil && f.Value.String() != f.DefValue {
		serverAddr = *serverAddrFlag
	}

	// POLL_INTERVAL (секунды в env/флагах, duration в JSON)
	pollSeconds := defaultPoll
	if fileCfg.PollInterval != "" {
		if d, err := time.ParseDuration(fileCfg.PollInterval); err == nil && d > 0 {
			pollSeconds = int(d.Seconds())
		}
	}
	if env := os.Getenv("POLL_INTERVAL"); env != "" {
		if parsed, err := strconv.Atoi(env); err == nil {
			pollSeconds = parsed
		}
	} else if f := flag.Lookup("p"); f != nil && f.Value.String() != f.DefValue {
		pollSeconds = *pollFlag
	}

	// REPORT_INTERVAL (секунды в env/флагах, duration в JSON)
	reportSeconds := defaultReport
	if fileCfg.ReportInterval != "" {
		if d, err := time.ParseDuration(fileCfg.ReportInterval); err == nil && d > 0 {
			reportSeconds = int(d.Seconds())
		}
	}
	if env := os.Getenv("REPORT_INTERVAL"); env != "" {
		if parsed, err := strconv.Atoi(env); err == nil {
			reportSeconds = parsed
		}
	} else if f := flag.Lookup("r"); f != nil && f.Value.String() != f.DefValue {
		reportSeconds = *reportFlag
	}

	// KEY
	key := ""
	if fileCfg.Key != "" {
		key = fileCfg.Key
	}
	if env := os.Getenv("KEY"); env != "" {
		key = env
	} else if f := flag.Lookup("k"); f != nil && f.Value.String() != f.DefValue {
		key = *keyFlag
	}

	// RATE_LIMIT
	rateLimit := defaultRateLimit
	if fileCfg.RateLimit != nil {
		rateLimit = *fileCfg.RateLimit
	}
	if env := os.Getenv("RATE_LIMIT"); env != "" {
		if parsed, err := strconv.Atoi(env); err == nil {
			rateLimit = parsed
		}
	} else if f := flag.Lookup("l"); f != nil && f.Value.String() != f.DefValue {
		rateLimit = *rateLimitFlag
	}

	// CRYPTO_KEY (путь до публичного ключа)
	cryptoKeyPath := ""
	if fileCfg.CryptoKey != "" {
		cryptoKeyPath = fileCfg.CryptoKey
	}
	if env := os.Getenv("CRYPTO_KEY"); env != "" {
		cryptoKeyPath = env
	} else if f := flag.Lookup("crypto-key"); f != nil && f.Value.String() != f.DefValue {
		cryptoKeyPath = *cryptoKeyFlag
	}

	config := &AgentConfig{
		ServerAddress:  normalizeServerAddress(serverAddr),
		PollInterval:   time.Duration(pollSeconds) * time.Second,
		ReportInterval: time.Duration(reportSeconds) * time.Second,
		Key:            key,
		RateLimit:      rateLimit,
		CryptoKeyPath:  cryptoKeyPath,
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
	cryptoInfo := "disabled"
	if c.CryptoKeyPath != "" {
		cryptoInfo = c.CryptoKeyPath
	}
	return fmt.Sprintf("Agent{Server: %s, Poll: %v, Report: %v, Key: %s, RateLimit: %d, CryptoKey: %s}",
		c.ServerAddress, c.PollInterval, c.ReportInterval, keyInfo, c.RateLimit, cryptoInfo)
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

// HasCryptoKeyPath возвращает true, если путь к публичному ключу задан
func (c *AgentConfig) HasCryptoKeyPath() bool {
	return c.CryptoKeyPath != ""
}

// loadAgentConfigFromFile читает и парсит JSON-файл конфигурации агента
func loadAgentConfigFromFile(path string) (*agentConfigFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return &agentConfigFile{}, nil
	}

	var cfg agentConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
