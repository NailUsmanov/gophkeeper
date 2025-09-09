// Package http содержит сетевые DTO (запросы/ответы) без бизнес-логики.
package http

import "time"

// RegisterRequest — входная модель для регистрации.
type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UserResponse — выходная модель для регистрации.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateSecretRequest это структура описывающая, какие данные получаем через сеть.
type CreateSecretRequest struct {
	Type  string         `json:"type"`
	Title string         `json:"title"`
	Data  map[string]any `json:"data"`
}

// UpdateSecretRequest описывает данные для обновления секрета.
type UpdateSecretRequest struct {
	Title   *string        `json:"title"`
	Data    map[string]any `json:"data"`
	Version int            `json:"version"`
}

// SecretResponse описывает какие данные с секрете мы хотим отправить обнатно.
type SecretResponse struct {
	ID        string         `json:"id"`         // id секрета
	Type      string         `json:"type"`       // "password" | "note" | "card" | "file"
	Title     string         `json:"title"`      // заголовок для сортировки секретов
	Data      map[string]any `json:"data"`       // полезные данные (по типу: login/password..., note.text, card.*, file.attachment_id и т.п.)
	Version   int            `json:"version"`    // версию будем использовать в optimistic locking
	CreatedAt string         `json:"created_at"` // время создания
	UpdatedAt string         `json:"updated_at"` // время обновления
}

// ListSecretResponse — ответ списка с пагинацией.
type ListSecretResponse struct {
	Items  []SecretResponse `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}
