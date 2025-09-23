// Package session используется для хранения токенов пользователя для сессии.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// fileName — как назовём файл сессии.
const fileName = "session.json"

type fileSession struct {
	AuthToken string `json:"auth_token"`
}

// ХУК для тестов: по умолчанию указывает на os.UserConfigDir.
var UserConfigDir = os.UserConfigDir

// DirPath возвращает каталог для хранения настроек/сессии.
func DirPath() (string, error) {
	root, err := UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("UserConfigDir: %w", err)
	}
	return root, nil
}

// Path — полный путь до файла сессии.
func Path() (string, error) {
	dir, err := DirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// SaveToken сохраняет токен в файл с правами 0600.
func SaveToken(token string) error {
	if token == "" {
		return fmt.Errorf("empty token")
	}
	dir, err := DirPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	p, _ := Path()
	payload := fileSession{AuthToken: token}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	// Записываем атомарно: сначала во временный файл, потом rename.
	tmp := p + ".part"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}

	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// LoadToken читает токен из файла, если он есть.
func LoadToken() (string, error) {
	p, err := Path()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // нет файла — нет сессии
		}
		return "", fmt.Errorf("read: %w", err)
	}
	var fs fileSession
	if err := json.Unmarshal(b, &fs); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	return fs.AuthToken, nil
}

// Clear удаляет файл сессии.
func Clear() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove: %w", err)
	}
	return nil
}
