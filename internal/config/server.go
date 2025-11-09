package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// ServerConfig содержит конфигурацию сервера
type ServerConfig struct {
	Address         string
	StoreInterval   time.Duration
	FileStoragePath string
	Restore         bool
}

func ParseServerConfig() (*ServerConfig, error) {
	const (
		defaultAddr        = "localhost:8080"
		defaultInterval    = 300 // секунды
		defaultStoragePath = "/tmp/metrics-db.json"
		defaultRestore     = true
	)

	var (
		addrFlag     = flag.String("a", "", "server address")
		intervalFlag = flag.Int("i", defaultInterval, "store interval in seconds")
		fileFlag     = flag.String("f", "", "file storage path")
		restoreFlag  = flag.Bool("r", defaultRestore, "restore from file on startup")
	)
	flag.Parse()

	// Приоритет: env -> flag -> default
	address := getEnvOrFlagString("ADDRESS", *addrFlag, defaultAddr)
	storeInterval := getEnvOrFlagInt("STORE_INTERVAL", *intervalFlag, defaultInterval)
	fileStoragePath := getEnvOrFlagString("FILE_STORAGE_PATH", *fileFlag, defaultStoragePath)
	restore := getEnvOrFlagBool("RESTORE", *restoreFlag, defaultRestore)

	config := &ServerConfig{
		Address:         address,
		StoreInterval:   time.Duration(storeInterval) * time.Second,
		FileStoragePath: fileStoragePath,
		Restore:         restore,
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	return config, nil
}

func getEnvOrFlagBool(envKey string, flagVal bool, defaultVal bool) bool {
	if envVal := os.Getenv(envKey); envVal != "" {
		if parsed, err := strconv.ParseBool(envVal); err == nil {
			return parsed
		}
	}
	// Если флаг явно указан (отличается от дефолта), используем его
	// Это работает только если дефолт != flagVal
	return flagVal
}

// validate проверяет корректность конфигурации сервера
func (c *ServerConfig) validate() error {
	if c.Address == "" {
		return fmt.Errorf("server address cannot be empty")
	}
	if c.FileStoragePath == "" {
		return fmt.Errorf("file storage path cannot be empty")
	}
	if c.StoreInterval < 0 {
		return fmt.Errorf("store interval cannot be negative")
	}
	return nil
}

// String возвращает строковое представление конфигурации
func (c *ServerConfig) String() string {
	return fmt.Sprintf("Server{Address: %s, StoreInterval: %v, FilePath: %s, Restore: %v}",
		c.Address, c.StoreInterval, c.FileStoragePath, c.Restore)
}

// LogConfig выводит конфигурацию в лог
func (c *ServerConfig) LogConfig() {
	log.Printf("Server configuration: %s", c.String())
}

// IsSyncMode возвращает true, если запись синхронная (interval = 0)
func (c *ServerConfig) IsSyncMode() bool {
	return c.StoreInterval == 0
}
