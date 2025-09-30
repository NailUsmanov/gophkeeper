// Package secret содержит интерфейсы бизнес-логики (SecretService) и интерфейсы доступа к данным (SecretRepository).
// Хендлеры вызывают методы SecretService, а тот внутри использует SecretRepository для работы с БД.
package secret

import (
	"context"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/google/uuid"
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

// SecretRepository — «как» хранится Secret (БД/файл и т.п.). Никаких бизнес-правил.
type SecretRepository interface {
	Insert(ctx context.Context, secret *Secret) error
	GetByID(ctx context.Context, ownerID, secretID string) (*Secret, error)                                       // фильтр по ownerID — защита от чтения чужого
	List(ctx context.Context, ownerID string, limit, offset int, filter SecretListFilter) ([]*Secret, int, error) // по умолчанию возвращает только не удалённые (DeletedAt == nil)
	Update(ctx context.Context, ownerID string, secret *Secret) error
	// SoftDelete(ctx context.Context, ownerID, secretID string, now time.Time) error
	// UpdatedAfter(ctx context.Context, ownerID string, since time.Time) ([]*Secret, error)
}

// ServiceCreate реализация сервиса. Она удовлетворяет локальным интерфейсам хендлера
type SecretService struct {
	repo SecretRepository
	now  time.Time
}

// NewServiceCreate конструктор сервиса.
func NewService(repo SecretRepository) *SecretService {
	return &SecretService{
		repo: repo,
		now:  time.Now(),
	}
}

// Create метод для создания секрета.
func (s *SecretService) Create(ctx context.Context, ownerID string, req CreateReq) (*Secret, error) {
	if req.Type == "" || req.Title == "" || req.Data == nil {
		return nil, models.NewInvalidInput(map[string]any{"fields": "type/title/data required"})
	}

	now := s.now
	Secret := &Secret{
		ID:        generateID(),
		OwnerID:   ownerID,
		Type:      req.Type,
		Title:     req.Title,
		Data:      req.Data,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Insert(ctx, Secret); err != nil {
		return nil, models.NewInternal(map[string]any{"cause": err.Error()})
	}
	return Secret, nil
}

// GetByID - метод для получения секрета по ID.
func (s *SecretService) GetByID(ctx context.Context, ownerID, secretID string) (*Secret, error) {
	secret, err := s.repo.GetByID(ctx, ownerID, secretID)
	if err != nil {
		if models.HasCode(err, models.ErrCodeNotFound.Error()) {
			return nil, models.NewNotFound(map[string]any{"id": "secretID"})
		}
		return nil, models.NewInternal(map[string]any{"cause": err.Error()})
	}
	if secret == nil {
		return nil, models.NewNotFound(map[string]any{"id": secretID})
	}

	res := &Secret{
		ID:        secretID,
		OwnerID:   secret.OwnerID,
		Type:      secret.Type,
		Title:     secret.Title,
		Data:      secret.Data,
		Version:   secret.Version,
		CreatedAt: secret.CreatedAt,
		UpdatedAt: secret.UpdatedAt,
	}
	return res, nil
}

// List возвращает список всех секретов с возможностью пагинации.
func (s *SecretService) List(ctx context.Context, ownerID string,
	limit, offset int, filter SecretListFilter) ([]*Secret, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	res, total, err := s.repo.List(ctx, ownerID, limit, offset, filter)
	if err != nil {
		return nil, 0, models.NewInternal(map[string]any{"cause": err.Error()})
	}

	if res == nil {
		res = []*Secret{}
	}
	return res, total, nil
}

// Update обновляет информацию по определенному секрету.
func (s *SecretService) Update(ctx context.Context, ownerID, secretID string, req UpdateReq) (*Secret, error) {

	// достаем старые данные.
	oldSecret, err := s.repo.GetByID(ctx, ownerID, secretID)
	if err != nil {
		return nil, models.NewNotFound(nil)
	}

	// если версии не сходятся - ошибка, иначе инкрементим.
	if oldSecret.Version != req.Version {
		return nil, models.NewConflict(nil)
	}
	newVersion := oldSecret.Version + 1

	if req.Data == nil {
		return nil, models.NewValidation(map[string]any{"data": "required"})
	}

	updatedAt := time.Now().UTC()
	newSecret := &Secret{
		ID:        secretID,
		OwnerID:   ownerID,
		Type:      oldSecret.Type,
		Data:      req.Data,
		Version:   newVersion,
		CreatedAt: oldSecret.CreatedAt,
		UpdatedAt: updatedAt,
	}
	// тайтл может быть не передан, тогда оставляем старый
	if req.Title != nil {
		newSecret.Title = *req.Title
	} else {
		newSecret.Title = oldSecret.Title
	}

	// выдаем апдейт.
	if err := s.repo.Update(ctx, ownerID, newSecret); err != nil {
		switch {
		case models.HasCode(err, models.ErrCodeNotFound.Error()):
			return nil, models.NewNotFound(nil)
		case models.HasCode(err, models.ErrCodeConflict.Error()):
			return nil, models.NewConflict(nil)
		default:
			return nil, models.NewInternal(map[string]any{"update": "failed"})
		}
	}
	return newSecret, nil
}

func generateID() string {
	return uuid.New().String()
}
