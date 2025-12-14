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
	DatabaseDSN     string
	Key             string // Ключ для проверки подписи
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
		databaseFlag = flag.String("d", "", "database DSN")
		keyFlag      = flag.String("k", "", "key for signing responses (SHA256)")
	)
	flag.Parse()

	// Приоритет: env -> flag -> default
	address := getEnvOrFlagString("ADDRESS", *addrFlag, defaultAddr)
	storeInterval := getEnvOrFlagInt("STORE_INTERVAL", *intervalFlag, defaultInterval)
	fileStoragePath := getEnvOrFlagString("FILE_STORAGE_PATH", *fileFlag, defaultStoragePath)
	databaseDSN := getEnvOrFlagString("DATABASE_DSN", *databaseFlag, "")
	key := getEnvOrFlagString("KEY", *keyFlag, "")

	// Передаем "r" как имя флага для Lookup
	restore := getEnvOrFlagBool("RESTORE", "r", *restoreFlag, defaultRestore)

	config := &ServerConfig{
		Address:         address,
		StoreInterval:   time.Duration(storeInterval) * time.Second,
		FileStoragePath: fileStoragePath,
		Restore:         restore,
		DatabaseDSN:     databaseDSN,
		Key:             key,
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	return config, nil
}

// getEnvOrFlagBool корректно обрабатывает bool флаги
// envKey - имя переменной окружения
// flagName - имя флага для flag.Lookup (например, "r")
// flagVal - текущее значение флага
// defaultVal - значение по умолчанию
func getEnvOrFlagBool(envKey string, flagName string, flagVal bool, defaultVal bool) bool {
	// Проверяем переменную окружения (приоритет 1)
	if envVal := os.Getenv(envKey); envVal != "" {
		if parsed, err := strconv.ParseBool(envVal); err == nil {
			return parsed
		}
	}

	// Проверяем, был ли флаг явно указан (приоритет 2)
	f := flag.Lookup(flagName)
	if f != nil && f.Value.String() != f.DefValue {
		// Флаг был явно указан, возвращаем его значение
		return flagVal
	}

	// Возвращаем дефолт (приоритет 3)
	return defaultVal
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
	keyInfo := "none"
	if c.Key != "" {
		keyInfo = "configured"
	}
	return fmt.Sprintf("Server{Address: %s, StoreInterval: %v, FilePath: %s, Restore: %v, Key: %s}",
		c.Address, c.StoreInterval, c.FileStoragePath, c.Restore, keyInfo)
}

// LogConfig выводит конфигурацию в лог
func (c *ServerConfig) LogConfig() {
	log.Printf("Server configuration: %s", c.String())
}

// IsSyncMode возвращает true, если запись синхронная (interval = 0)
func (c *ServerConfig) IsSyncMode() bool {
	return c.StoreInterval == 0
}

// HasKey возвращает true, если ключ для подписи установлен
func (c *ServerConfig) HasKey() bool {
	return c.Key != ""
}
