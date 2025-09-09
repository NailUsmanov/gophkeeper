// Package memory реаизует простое хранилище сессий (токена).
//
// Используется как SessionStore для opaque-токенов.
package memory

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrNotFound возвращается, если в хранилище не найден токен.
var ErrNotFound = errors.New("session not found")

type record struct {
	userID    string    // ID пользователя, которому принадлежит токен
	expiresAt time.Time // время, когда токен истекает
	revoked   bool      // признак отзыва токена
}

// Store - хранилище записей токенов.
type Store struct {
	mu   sync.RWMutex
	data map[string]record
}

// NewStore создаёт новый пустой Store.
func NewStore() *Store {
	return &Store{data: make(map[string]record)}
}

// Create сохраняет в Store запись о токене.
//
// Аргументы:
//
//	token — строка токена (ключ);
//	userID — ID пользователя;
//	expiresAt — время истечения токена.
func (s *Store) Create(ctx context.Context, token, userID string, expiresAt time.Time) error {
	s.mu.Lock()
	s.data[token] = record{userID: userID, expiresAt: expiresAt, revoked: false}
	s.mu.Unlock()
	return nil
}

// Get возвращает данные о токене из Store.
//
// Результаты:
//
//	userID    — ID пользователя;
//	expiresAt — время окончания действия токена;
//	revoked   — true, если токен отозван;
//	error     — ErrNotFound, если токен отсутствует.
func (s *Store) Get(ctx context.Context, token string) (string, time.Time, bool, error) {
	s.mu.RLock()
	rec, ok := s.data[token]
	s.mu.RUnlock()
	if !ok {
		return "", time.Time{}, false, ErrNotFound
	}
	return rec.userID, rec.expiresAt, rec.revoked, nil
}

// Revoke помечает токен как отозванный.
// Если токен отсутствует — возвращает ErrNotFound.
func (s *Store) Revoke(ctx context.Context, token string) error {
	s.mu.Lock()
	rec, ok := s.data[token]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	rec.revoked = true
	s.data[token] = rec
	s.mu.Unlock()
	return nil
}
