package config_test

import (
	"testing"

	"github.com/NailUsmanov/gophkeeper/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestNewConfig_Success_AllEnvSet(t *testing.T) {
	t.Setenv("SERVER_ADDR", ":9090")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/gk?sslmode=disable")
	t.Setenv("ATTACHMENTS_DIR", "/var/gk/attachments")

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	require.Equal(t, ":9090", cfg.ServerAddr)
	require.Equal(t, "postgres://user:pass@localhost:5432/gk?sslmode=disable", cfg.DatabaseURL)
	require.Equal(t, "/var/gk/attachments", cfg.AttachmentDir)
}

func TestNewConfig_DefaultsApplied(t *testing.T) {
	// Задаём только обязательную переменную
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/gk?sslmode=disable")

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	// Значения по умолчанию из тегов envDefault
	require.Equal(t, ":8080", cfg.ServerAddr)
	require.Equal(t, "./var/attachments", cfg.AttachmentDir)
	require.Equal(t, "postgres://u:p@localhost:5432/gk?sslmode=disable", cfg.DatabaseURL)
}

func TestNewConfig_MissingDatabaseURL_ReturnsError(t *testing.T) {
	// Специально ничего не ставим для DATABASE_URL
	// (t.Setenv не нужен — отсутствие переменной и проверяем)

	cfg, err := config.NewConfig()
	require.Nil(t, cfg)
	require.Error(t, err, "ожидаем ошибку, т.к. DATABASE_URL помечен как required")
}
