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

func ParseServerConfig() (*ServerConfig, error) {
	const defaultAddr = "localhost:8080"
	var addrFlag = flag.String("a", "", "server address")
	flag.Parse()

	address := getEnvOrFlagString("ADDRESS", *addrFlag, defaultAddr)
	config := &ServerConfig{Address: address}

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
