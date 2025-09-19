// Package postgres используется для связи с БД.
//
// attachment_repo используется для работы с метаданными в БД.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/storage/postgres"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// AttachmentRepository структура для инициализации БД.
type AttachmentRepository struct {
	db postgres.PgxPool
}

// NewAttachmentRepository - конструктор AttachmentRepository.
func NewAttachmentRepository(db postgres.PgxPool) *AttachmentRepository {
	return &AttachmentRepository{db: db}
}

// Insert реализует вставку метаданных AttachmentMeta в БД.
func (a *AttachmentRepository) Insert(ctx context.Context, meta *models.AttachmentMeta) error {
	// 1. Вставляем данные в БД.
	ct, err := a.db.Exec(ctx, postgres.InsertAttachmentQuery,
		meta.ID,
		meta.FileName,
		meta.SecretID,
		meta.OwnerID,
		meta.Size,
		meta.ContentType,
		meta.CreatedAt.UTC(),
	)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			switch pgErr.Code {
			case pgerrcode.UniqueViolation:
				return models.NewConflict(map[string]any{"id": "duplicate"})
			case pgerrcode.CheckViolation:
				return models.NewBadRequest(map[string]any{"check": pgErr.Message})
			case pgerrcode.ForeignKeyViolation:
				return models.NewBadRequest(map[string]any{"fk": pgErr.Message})
			}
		}
		return models.NewInternal(map[string]any{"insert": err.Error()})
	}
	// 2. Проверяем, вставилась ли одна строка.
	if ct.RowsAffected() != 1 {
		return models.NewInternal(map[string]any{"insert": "no rows affected"})
	}
	return nil

}

// GetByID производит поиск и выдачу метаданных AttachmentMeta в БД по ID.
func (a *AttachmentRepository) GetByID(ctx context.Context, attachmentID string) (*models.AttachmentMeta, error) {
	// 1. Создаем локальные переменные для чтения сырых данных из БД.
	var meta models.AttachmentMeta
	// 2. Делаем запрос к БД и сканим сразу в доменную модель.
	err := a.db.QueryRow(ctx, postgres.GetByIDAttachmentQuery, attachmentID).Scan(
		&meta.ID,
		&meta.FileName,
		&meta.SecretID,
		&meta.OwnerID,
		&meta.Size, // int64
		&meta.ContentType,
		&meta.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.NewNotFound(map[string]any{"id": attachmentID})
		}
		return nil, models.NewInternal(map[string]any{"select": err.Error()})
	}
	meta.CreatedAt = meta.CreatedAt.UTC()

	return &meta, nil
}

// Delete удаляет метаданные из БД.
func (a *AttachmentRepository) Delete(ctx context.Context, attachmentID string) error {

	// 1. Выполняем запрос к БД.
	ct, err := a.db.Exec(ctx, postgres.DeleteAttachmentQuery, attachmentID)
	if err != nil {
		return models.NewInternal(map[string]any{"delete": err.Error()})
	}
	// 2. Проверяем, что реально что-то удалилось
	if ct.RowsAffected() == 0 {
		return models.NewNotFound(map[string]any{"id": attachmentID})
	}
	return nil
}

// List возвращает массив метаданных пользователя с возможностью пагинации.
func (a *AttachmentRepository) List(ctx context.Context, ownerID, secretID string, limit, offset int) ([]models.AttachmentMeta, int, error) {

	// 1. Получаем общее число метаданных файлов пользователя.
	total, err := a.countAttachment(ctx, ownerID, secretID)
	if err != nil {
		return nil, 0, err
	}
	// 2. Выполняю запрос к БД.
	rows, err := a.db.Query(ctx, postgres.ListAttachmentQuery, secretID, ownerID, limit, offset)
	if err != nil {
		return nil, 0, models.NewInternal(map[string]any{"select list": err.Error()})
	}
	defer rows.Close()
	// 3. Проходимся по циклу и добавляем в массив структуры
	capacity := min(total, limit)
	result := make([]models.AttachmentMeta, 0, capacity)
	for rows.Next() {
		var m models.AttachmentMeta
		if err := rows.Scan(
			&m.ID,
			&m.FileName,
			&m.SecretID,
			&m.OwnerID,
			&m.Size,
			&m.ContentType,
			&m.CreatedAt,
		); err != nil {
			return nil, 0, models.NewInternal(map[string]any{"scan": err.Error()})
		}
		m.CreatedAt = m.CreatedAt.UTC()
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, models.NewInternal(map[string]any{"rows": err.Error()})
	}
	return result, total, nil
}

func (a *AttachmentRepository) countAttachment(ctx context.Context, ownerID, secretID string) (int, error) {
	var n int
	if err := a.db.QueryRow(ctx, postgres.CountAttachmentQuery, ownerID, secretID).Scan(&n); err != nil {
		return 0, models.NewInternal(map[string]any{"count": err.Error()})
	}
	return n, nil
}

// Ping проверяет доступность подключения к базе.
// Возвращает ошибку, если Postgres недоступен или ctx отменён.
func (a *AttachmentRepository) Ping(ctx context.Context) error {
	// Проверяем отмену контекста
	if err := ctx.Err(); err != nil {
		return err
	}

	// Выполняем самый лёгкий запрос
	if err := a.db.Ping(ctx); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}

	return nil
}
