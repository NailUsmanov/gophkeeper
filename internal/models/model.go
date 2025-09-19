// Package models содержит сущности и константы, которые являются общими для всего проекта.
package models

import (
	"time"
)

// User — структура пользователя системы.
type User struct {
	ID           string // ID пользователя
	Email        string
	CreatedAt    time.Time // когда создали пользователя
	PasswordHash string
}

// SecretType — тип секрета.
type SecretType string

const (
	SecretPassword SecretType = "password"
	SecretNote     SecretType = "note"
	SecretCard     SecretType = "card"
	SecretFile     SecretType = "file"
)

// AttachmentMeta — метаданные вложения (файла). Байты самого файла лежат в Storage.
type AttachmentMeta struct {
	ID          string // id файла
	FileName    string // имя файла
	SecretID    string // к какому секрету привязано
	OwnerID     string // к кому привязано по ID
	Size        int64
	ContentType string // исходный тип файла
	CreatedAt   time.Time
}
