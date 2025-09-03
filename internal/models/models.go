// Package models содержит сущности и константы, которые являются общими для всего проекта.
package models

import (
	"time"
)

// User — структура пользователя системы.
type User struct {
	ID        string // ID пользователя
	Email     string
	CreatedAt time.Time // когда создали пользователя
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
	SecretID    string // к какому секрету привязано
	Size        int
	ContentType string // исходный тип файла
	CheckSum    string // контрольная сумма (например, hex SHA-256)
	CreatedAt   time.Time
}
