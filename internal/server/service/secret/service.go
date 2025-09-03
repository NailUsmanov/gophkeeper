// Package secret содержит интерфейсы бизнес-логики (SecretService) и интерфейсы доступа к данным (SecretRepository).
// Хендлеры вызывают методы SecretService, а тот внутри использует SecretRepository для работы с БД.
package secret

import (
	"context"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
)

// Secret — доменная сущность, которая описывает что такое Секрет в системе.
type Secret struct {
	ID        string            // id секрета
	OwnerID   string            // id владельца секрета
	Type      models.SecretType // "password" | "note" | "card" | "file"
	Title     string            // заголовок для сортировки секретов
	Data      map[string]any    // полезные данные (по типу: login/password..., note.text, card.*, file.attachment_id и т.п.)
	Version   int               // версию будем использовать в optimistic locking
	CreatedAt time.Time         // время создания
	UpdatedAt time.Time         // время обновления
	DeletedAt *time.Time        // soft delete: nil — живой, не nil — помечён к удалению
}

// CreateReq — то, что приходит из API при создании секрета.
// Сервис сам превратит это в доменную модель Secret и заполнит служебные поля.
type CreateReq struct {
	Type  models.SecretType // тип секрета
	Title string            // заголовок
	Data  map[string]any    // тело секрета по заголовку
}

// UpdateReq — то, что приходит из API при обновлении.
// Включает обязательную версию для optimistic locking.
type UpdateReq struct {
	Title   *string        // опционально изменить заголовок
	Data    map[string]any // новое содержимое
	Version int            // версия, которую клиент редактировал (N)
}

// SecretListFilter — фильтры для списка, можем фильтровать по типу секрета или по дате обновления.
type SecretListFilter struct {
	Type         *models.SecretType // фильтр по типу
	UpdatedAfter *time.Time         // для sync (вернуть изменённые позже этого времени)
}

// SecretService — бизнес-правила: проверка владельца, валидация по типам,
// маскирование полей карты в ответах, optimistic locking и т.д.
type SecretService interface {
	Create(ctx context.Context, ownerID string, req CreateReq) (*Secret, error)
	GetByID(ctx context.Context, ownerID, secretID string) (*Secret, error)
	List(ctx context.Context, ownerID string, limit, offset int, filter SecretListFilter) (items []*Secret, total int, err error)
	Update(ctx context.Context, ownerID, secretID string, req UpdateReq) (*Secret, error)
	Delete(ctx context.Context, ownerID, secretID string) error
}

// SecretRepository — «как» хранится Secret (БД/файл и т.п.). Никаких бизнес-правил.
type SecretRepository interface {
	Insert(ctx context.Context, secret *Secret) error
	GetByID(ctx context.Context, ownerID, secretID string) (*Secret, error)                                       // фильтр по ownerID — защита от чтения чужого
	List(ctx context.Context, ownerID string, limit, offset int, filter SecretListFilter) ([]*Secret, int, error) // по умолчанию возвращает только не удалённые (DeletedAt == nil)
	Update(ctx context.Context, ownerID string, secret *Secret) error
	SoftDelete(ctx context.Context, ownerID, secretID string, now time.Time) error
	UpdatedAfter(ctx context.Context, ownerID string, since time.Time) ([]*Secret, error)
}
