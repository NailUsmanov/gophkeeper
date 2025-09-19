// Package config обеспечивает конфигурацию приложения.
// Включает настройки адреса сервера, базового URL, хранилища.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v6"
)

// Config описывает, адрес HTTP-сервера, DSN Postgres, директорию вложений.
type Config struct {
	ServerAddr    string `env:"SERVER_ADDR" envDefault:":8080"`                 // адрес HTTP-сервера, напр. ":8080"
	DatabaseURL   string `env:"DATABASE_URL,required"`                          // DSN Postgres
	AttachmentDir string `env:"ATTACHMENTS_DIR" envDefault:"./var/attachments"` // директория для вложений, напр. "./var/attachments"
}

// NewConfig - конструктор конфига.
func NewConfig() (*Config, error) {
	var cfg Config
	// Парсим переменные окружения в Конфиг.
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	return &cfg, nil
}
