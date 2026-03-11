package config

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"encoding/json"
)

// ServerConfig содержит конфигурацию сервера
type ServerConfig struct {
	Address         string
	StoreInterval   time.Duration
	FileStoragePath string
	Restore         bool
	DatabaseDSN     string
	Key             string // Ключ для проверки подписи
	AuditFile       string // Путь к файлу аудита (если пусто - выключено)
	AuditURL        string // URL приёмника аудита (если пусто - выключено)
	CryptoKeyPath   string // Путь к файлу с приватным ключом (RSA)
	TrustedSubnet   string // Доверенная подсеть в CIDR-формате
}

// serverConfigFile описывает формат JSON-конфигурации сервера
type serverConfigFile struct {
	Address       string `json:"address"`
	Restore       *bool  `json:"restore"`
	StoreInterval string `json:"store_interval"`
	StoreFile     string `json:"store_file"`
	DatabaseDSN   string `json:"database_dsn"`
	Key           string `json:"key"`
	AuditFile     string `json:"audit_file"`
	AuditURL      string `json:"audit_url"`
	CryptoKey     string `json:"crypto_key"`
	TrustedSubnet string `json:"trusted_subnet"`
}

func ParseServerConfig() (*ServerConfig, error) {
	const (
		defaultAddr        = "localhost:8080"
		defaultInterval    = 300 // секунды
		defaultStoragePath = "/tmp/metrics-db.json"
		defaultRestore     = true
	)

	var (
		configPath   string
		addrFlag     = flag.String("a", "", "server address")
		intervalFlag = flag.Int("i", defaultInterval, "store interval in seconds")
		fileFlag     = flag.String("f", "", "file storage path")
		restoreFlag  = flag.Bool("r", defaultRestore, "restore from file on startup")
		databaseFlag = flag.String("d", "", "database DSN")
		keyFlag       = flag.String("k", "", "key for signing responses (SHA256)")
		auditFile     = flag.String("audit-file", "", "audit file path (empty disables audit file sink)")
		auditURL      = flag.String("audit-url", "", "audit remote url (empty disables audit http sink)")
		cryptoKey     = flag.String("crypto-key", "", "path to private key file for asymmetric encryption (RSA)")
		trustedSubnet = flag.String("t", "", "trusted subnet in CIDR format")
	)

	// Путь к файлу конфигурации: флаги -c / -config
	flag.StringVar(&configPath, "c", "", "path to configuration file (JSON)")
	flag.StringVar(&configPath, "config", "", "path to configuration file (JSON)")

	flag.Parse()

	// Если путь не задан флагом, пробуем переменную окружения CONFIG
	if configPath == "" {
		configPath = os.Getenv("CONFIG")
	}

	var fileCfg serverConfigFile
	if configPath != "" {
		cfgFromFile, err := loadServerConfigFromFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load server config file: %w", err)
		}
		if cfgFromFile != nil {
			fileCfg = *cfgFromFile
		}
	}

	// Приоритет опций: env -> flag -> config file -> default

	// ADDRESS
	address := defaultAddr
	if fileCfg.Address != "" {
		address = fileCfg.Address
	}
	if env := os.Getenv("ADDRESS"); env != "" {
		address = env
	} else if f := flag.Lookup("a"); f != nil && f.Value.String() != f.DefValue {
		address = *addrFlag
	}

	// STORE_INTERVAL (секунды в env/флагах, duration в JSON)
	storeIntervalSeconds := defaultInterval
	if fileCfg.StoreInterval != "" {
		if d, err := time.ParseDuration(fileCfg.StoreInterval); err == nil {
			if d >= 0 {
				storeIntervalSeconds = int(d.Seconds())
			}
		}
	}
	if env := os.Getenv("STORE_INTERVAL"); env != "" {
		if parsed, err := strconv.Atoi(env); err == nil {
			storeIntervalSeconds = parsed
		}
	} else if f := flag.Lookup("i"); f != nil && f.Value.String() != f.DefValue {
		storeIntervalSeconds = *intervalFlag
	}

	// FILE_STORAGE_PATH / store_file
	fileStoragePath := defaultStoragePath
	if fileCfg.StoreFile != "" {
		fileStoragePath = fileCfg.StoreFile
	}
	if env := os.Getenv("FILE_STORAGE_PATH"); env != "" {
		fileStoragePath = env
	} else if f := flag.Lookup("f"); f != nil && f.Value.String() != f.DefValue {
		fileStoragePath = *fileFlag
	}

	// DATABASE_DSN
	databaseDSN := ""
	if fileCfg.DatabaseDSN != "" {
		databaseDSN = fileCfg.DatabaseDSN
	}
	if env := os.Getenv("DATABASE_DSN"); env != "" {
		databaseDSN = env
	} else if f := flag.Lookup("d"); f != nil && f.Value.String() != f.DefValue {
		databaseDSN = *databaseFlag
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

	// AUDIT_FILE
	auditFilePath := ""
	if fileCfg.AuditFile != "" {
		auditFilePath = fileCfg.AuditFile
	}
	if env := os.Getenv("AUDIT_FILE"); env != "" {
		auditFilePath = env
	} else if f := flag.Lookup("audit-file"); f != nil && f.Value.String() != f.DefValue {
		auditFilePath = *auditFile
	}

	// AUDIT_URL
	auditURLValue := ""
	if fileCfg.AuditURL != "" {
		auditURLValue = fileCfg.AuditURL
	}
	if env := os.Getenv("AUDIT_URL"); env != "" {
		auditURLValue = env
	} else if f := flag.Lookup("audit-url"); f != nil && f.Value.String() != f.DefValue {
		auditURLValue = *auditURL
	}

	// CRYPTO_KEY
	cryptoKeyPath := ""
	if fileCfg.CryptoKey != "" {
		cryptoKeyPath = fileCfg.CryptoKey
	}
	if env := os.Getenv("CRYPTO_KEY"); env != "" {
		cryptoKeyPath = env
	} else if f := flag.Lookup("crypto-key"); f != nil && f.Value.String() != f.DefValue {
		cryptoKeyPath = *cryptoKey
	}

	// TRUSTED_SUBNET
	trustedSubnetVal := ""
	if fileCfg.TrustedSubnet != "" {
		trustedSubnetVal = fileCfg.TrustedSubnet
	}
	if env := os.Getenv("TRUSTED_SUBNET"); env != "" {
		trustedSubnetVal = env
	} else if f := flag.Lookup("t"); f != nil && f.Value.String() != f.DefValue {
		trustedSubnetVal = *trustedSubnet
	}

	// RESTORE (bool) — JSON: restore, env: RESTORE, flag: -r
	restore := defaultRestore
	if fileCfg.Restore != nil {
		restore = *fileCfg.Restore
	}
	if env := os.Getenv("RESTORE"); env != "" {
		if parsed, err := strconv.ParseBool(env); err == nil {
			restore = parsed
		}
	} else if f := flag.Lookup("r"); f != nil && f.Value.String() != f.DefValue {
		restore = *restoreFlag
	}

	config := &ServerConfig{
		Address:         address,
		StoreInterval:   time.Duration(storeIntervalSeconds) * time.Second,
		FileStoragePath: fileStoragePath,
		Restore:         restore,
		DatabaseDSN:     databaseDSN,
		Key:             key,
		AuditFile:       auditFilePath,
		AuditURL:        auditURLValue,
		CryptoKeyPath:   cryptoKeyPath,
		TrustedSubnet:   trustedSubnetVal,
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
	cryptoInfo := "disabled"
	if c.CryptoKeyPath != "" {
		cryptoInfo = c.CryptoKeyPath
	}
	auditFileInfo := "disabled"
	if c.AuditFile != "" {
		auditFileInfo = c.AuditFile
	}
	auditURLInfo := "disabled"
	if c.AuditURL != "" {
		auditURLInfo = c.AuditURL
	}

	trustedSubnetInfo := "disabled"
	if c.TrustedSubnet != "" {
		trustedSubnetInfo = c.TrustedSubnet
	}

	return fmt.Sprintf("Server{Address: %s, StoreInterval: %v, FilePath: %s, Restore: %v, Key: %s, CryptoKey: %s, AuditFile: %s, AuditURL: %s, TrustedSubnet: %s}",
		c.Address, c.StoreInterval, c.FileStoragePath, c.Restore, keyInfo, cryptoInfo, auditFileInfo, auditURLInfo, trustedSubnetInfo)
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

// HasCryptoKeyPath возвращает true, если путь к приватному ключу задан
func (c *ServerConfig) HasCryptoKeyPath() bool {
	return c.CryptoKeyPath != ""
}

// loadServerConfigFromFile читает и парсит JSON-файл конфигурации сервера
func loadServerConfigFromFile(path string) (*serverConfigFile, error) {
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
		return &serverConfigFile{}, nil
	}

	var cfg serverConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
