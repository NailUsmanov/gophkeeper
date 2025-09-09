// Package token реализует работу с opaque-токенами.
//
// Opaque-токен — это случайная строка (байты → hex), которая хранится
// в SessionStore вместе с userID, временем истечения и статусом (revoked).
//
// Подходит для простых и безопасных схем аутентификации: проверка
// выполняется на сервере, токен невозможно «подделать» на клиенте.
package token

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// SessionStore описывает интерфейс для хранения сессий.
// Конкретная реализация в in-memory.
type SessionStore interface {
	Create(ctx context.Context, token, userID string, expiresAt time.Time) error
	Get(ctx context.Context, token string) (userID string, expiresAt time.Time, revoked bool, err error)
	Revoke(ctx context.Context, token string) error
}

// OpaqueManager управляет выдачей, проверкой и отзывом opaque-токенов.
//
// Хранение состояния делегируется SessionStore.
// TTL задаётся при создании и определяет срок жизни токена.
type OpaqueManager struct {
	store SessionStore
	ttl   time.Duration
}

// NewOpaqueManager создаёт новый менеджер токенов с указанным хранилищем и TTL.
func NewOpaqueManager(store SessionStore, ttl time.Duration) *OpaqueManager {
	return &OpaqueManager{store: store, ttl: ttl}
}

// Issue генерирует новый токен для пользователя и сохраняет его в SessionStore.
// Возвращает строку токена.
func (m *OpaqueManager) Issue(ctx context.Context, userID string) (string, error) {
	tok, err := randomToken(32) // 32 байта
	if err != nil {
		return "", err
	}
	if err := m.store.Create(ctx, tok, userID, time.Now().UTC().Add(m.ttl)); err != nil {
		return "", err
	}
	return tok, nil
}

// Validate проверяет токен через SessionStore.
// Возвращает userID, если токен существует, не истёк и не отозван.
// В противном случае возвращает ошибку.
func (m *OpaqueManager) Validate(ctx context.Context, token string) (string, error) {
	userID, exp, revoked, err := m.store.Get(ctx, token)
	if err != nil {
		return "", err
	}
	if revoked || time.Now().UTC().After(exp) {
		return "", errors.New("token invalid")
	}
	return userID, nil
}

// Revoke отзывает токен: помечает его как недействительный в SessionStore.
func (m *OpaqueManager) Revoke(ctx context.Context, token string) error {
	return m.store.Revoke(ctx, token)
}

// randomToken генерирует случайный токен длиной n байт и возвращает его в hex-виде.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
