package config

import (
	"flag"
	"fmt"
	"log"
)

// ServerConfig содержит конфигурацию сервера
type ServerConfig struct {
	Address string
}

// ParseServerConfig парсит флаги командной строки для сервера
func ParseServerConfig() (*ServerConfig, error) {
	config := &ServerConfig{}

	// Определяем флаги с значениями по умолчанию
	flag.StringVar(&config.Address, "a", "localhost:8080", "server address")

	// Парсим флаги
	flag.Parse()

	// Проверяем, что нет неизвестных аргументов
	if flag.NArg() > 0 {
		return nil, fmt.Errorf("unknown arguments: %v", flag.Args())
	}

	// Валидируем конфигурацию
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return config, nil
}

// validate проверяет корректность конфигурации сервера
func (c *ServerConfig) validate() error {
	if c.Address == "" {
		return fmt.Errorf("server address cannot be empty")
	}
	return nil
}

// String возвращает строковое представление конфигурации
func (c *ServerConfig) String() string {
	return fmt.Sprintf("Server{Address: %s}", c.Address)
}

// LogConfig выводит конфигурацию в лог
func (c *ServerConfig) LogConfig() {
	log.Printf("Server configuration: %s", c.String())
}
