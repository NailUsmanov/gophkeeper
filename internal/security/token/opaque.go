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

// ---- добавили строчку ниже ----
var genToken = randomToken

// --------------------------------

type SessionStore interface {
	Create(ctx context.Context, token, userID string, expiresAt time.Time) error
	Get(ctx context.Context, token string) (userID string, expiresAt time.Time, revoked bool, err error)
	Revoke(ctx context.Context, token string) error
}

type OpaqueManager struct {
	store SessionStore
	ttl   time.Duration
}

func NewOpaqueManager(store SessionStore, ttl time.Duration) *OpaqueManager {
	return &OpaqueManager{store: store, ttl: ttl}
}

func (m *OpaqueManager) Issue(ctx context.Context, userID string) (string, error) {
	// ---- было: tok, err := randomToken(32)
	tok, err := genToken(32)
	// --------------------------------------
	if err != nil {
		return "", err
	}
	if err := m.store.Create(ctx, tok, userID, time.Now().UTC().Add(m.ttl)); err != nil {
		return "", err
	}
	return tok, nil
}

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

func (m *OpaqueManager) Revoke(ctx context.Context, token string) error {
	return m.store.Revoke(ctx, token)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
