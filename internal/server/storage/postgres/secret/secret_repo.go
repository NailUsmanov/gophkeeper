// Package postgres используется для связи с БД.
//
// secret_repo.go  производит вставку, получение, обновление секрета.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgerrcode"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/service/secret"
	"github.com/NailUsmanov/gophkeeper/internal/server/storage/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SecretRepo структура для инициализации базы данных.
type SecretRepo struct {
	db postgres.PgxPool
}

// NewSecretRepo создает экземпляр SecretRepo
func NewSecretRepo(db postgres.PgxPool) *SecretRepo {
	return &SecretRepo{
		db: db,
	}
}

// Insert производит вставку Секрета в БД.
func (r *SecretRepo) Insert(ctx context.Context, s *secret.Secret) error {
	// 1. Сериализуем data → jsonb
	dataBytes, err := json.Marshal(s.Data)
	if err != nil {
		return models.NewValidation(map[string]any{"data": "invalid json"})
	}
	// 2. Вставляем в БД.
	ct, err := r.db.Exec(ctx, postgres.QueryInsertSecret,
		s.ID,
		s.OwnerID,
		string(s.Type),
		s.Title,
		dataBytes,
		s.Version,
		s.CreatedAt.UTC(),
		s.UpdatedAt.UTC(),
	)
	if err != nil {
		// проверяем нет ли уже такого id в базе.
		// Postgres возвращает ошибку, приводим ее к pgconn.PgError,
		// сравниваем ее код с pgerrcode.UniqueViolation(23505)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return models.NewConflict(map[string]any{"id": "duplicate"})
		}
		return models.NewInternal(map[string]any{"insert": err.Error()})
	}
	// 3. Гарантируем, что вставлена 1 строка
	if ct.RowsAffected() != 1 {
		return models.NewInternal(map[string]any{"insert": "no rows affected"})
	}

	return nil

}

// GetByID возвращает секрет из БД по его ID.
func (r *SecretRepo) GetByID(ctx context.Context, ownerID, secretID string) (*secret.Secret, error) {
	// 1. Локальные переменные для чтения «сырых» значений из БД.
	//    Сканим в string/int/time.Time/[]byte, а потом преобразуем.
	var (
		id, owID, typ, title string
		dataBytes            []byte
		ver                  int
		createdAt, updatedAt time.Time
	)
	// 2. Делаем SELECT из БД. Фильтруем по owner_id и исключаем удалённые (deleted_at IS NULL).
	err := r.db.QueryRow(ctx, postgres.QueryGetByIDSecret, secretID, ownerID).Scan(
		&id, &owID, &typ, &title, &dataBytes, &ver, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.NewNotFound(map[string]any{"id": secretID})
		}
		return nil, models.NewInternal(map[string]any{"select": err.Error()})
	}
	// 3. Парсим jsonb в map[string]any.
	var data map[string]any
	if len(dataBytes) > 0 {
		if uerr := json.Unmarshal(dataBytes, &data); uerr != nil {
			return nil, models.NewInternal(map[string]any{"data": "invalid json in db"})
		}
	}
	// 4. Собираем доменную модель Secret.
	s := &secret.Secret{
		ID:        id,
		OwnerID:   owID,
		Type:      models.SecretType(typ),
		Title:     title,
		Data:      data,
		Version:   ver,
		CreatedAt: createdAt.UTC(),
		UpdatedAt: updatedAt.UTC(),
		DeletedAt: nil,
	}
	return s, nil
}

// List позволяет получить список Секретов пользователя с пагинацией.
func (r *SecretRepo) List(ctx context.Context, ownerID string, limit, offset int, filter secret.SecretListFilter) ([]*secret.Secret, int, error) {
	// 1. Массив параметров для запроса.
	args := []any{ownerID}
	// Будем далее добавлять доп. фильтры с новыми плейсхолдорами, начинать будем со второго.
	arg := 2
	query := postgres.QueryListSecret
	// Если нужно добавлять фильтр по Типу
	if filter.Type != nil {
		query += " AND type = $" + strconv.Itoa(arg)
		args = append(args, string(*filter.Type))
		arg++
	}
	// Если нужно добавлять фильтр по Обновлению
	if filter.UpdatedAfter != nil {
		query += " AND updated_at > $" + strconv.Itoa(arg)
		args = append(args, filter.UpdatedAfter.UTC())
		arg++
	}

	// Итоговый запрос c фильтрацией и добавлением лимита и смещения.
	query += ` ORDER BY updated_at DESC, id
		LIMIT $` + strconv.Itoa(arg) + ` OFFSET $` + strconv.Itoa(arg+1)
	args = append(args, limit, offset)

	// 2. Выполняем запрос к БД.
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, models.NewInternal(map[string]any{"list": err.Error()})
	}
	defer rows.Close()

	// 3. Идем по строкам результата rows.
	items := make([]*secret.Secret, 0, limit)
	for rows.Next() {
		// Сырые переменные под типы БД
		var (
			id, oid, typ, title  string
			dataBytes            []byte
			ver                  int
			createdAt, updatedAt time.Time
		)

		if err := rows.Scan(&id, &oid, &typ, &title, &dataBytes, &ver, &createdAt, &updatedAt); err != nil {
			return nil, 0, models.NewInternal(map[string]any{"scan": err.Error()})
		}
		var data map[string]any
		if len(dataBytes) > 0 {
			if uerr := json.Unmarshal(dataBytes, &data); uerr != nil {
				return nil, 0, models.NewInternal(map[string]any{"data": "invalid json in db"})
			}
		}
		// 4. Собираем доменную модель.
		items = append(items, &secret.Secret{
			ID:        id,
			OwnerID:   oid,
			Type:      models.SecretType(typ),
			Title:     title,
			Data:      data,
			Version:   ver,
			CreatedAt: createdAt.UTC(),
			UpdatedAt: updatedAt.UTC(),
			DeletedAt: nil,
		})
	}
	// 5. Проверяем ошибку итератора.
	if err := rows.Err(); err != nil {
		return nil, 0, models.NewInternal(map[string]any{"rows": err.Error()})
	}
	// 6. Вызываем count для подсчета всех секретов.
	total, err := r.countSecret(ctx, ownerID, filter)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// countSecret считает количество имеющихся Секретов пользователя.
func (r *SecretRepo) countSecret(ctx context.Context, ownerID string, filter secret.SecretListFilter) (int, error) {
	base := postgres.QueryCountSecret
	args := []any{ownerID}
	arg := 2
	if filter.Type != nil {
		base += " AND type = $" + strconv.Itoa(arg)
		args = append(args, string(*filter.Type))
		arg++
	}
	if filter.UpdatedAfter != nil {
		base += " AND updated_at > $" + strconv.Itoa(arg)
		args = append(args, filter.UpdatedAfter.UTC())
		arg++
	}

	var n int
	if err := r.db.QueryRow(ctx, base, args...).Scan(&n); err != nil {
		return 0, models.NewInternal(map[string]any{"count": err.Error()})
	}
	return n, nil
}

// Update обновляет Секрет пользователя по ID.
func (r *SecretRepo) Update(ctx context.Context, ownerID string, s *secret.Secret) error {
	// 1. Сериализуем новое содержимое
	dataBytes, err := json.Marshal(s.Data)
	if err != nil {
		return models.NewNotFound(map[string]any{"id": s.ID})
	}
	// 2) выполняем UPDATE: новая версия = s.Version, старая в WHERE = s.Version-1
	ct, err := r.db.Exec(ctx, postgres.QueryUpdate,
		s.Title,
		dataBytes,
		s.Version,
		s.UpdatedAt.UTC(),
		s.ID,
		ownerID,
		s.Version-1)
	if err != nil {
		return models.NewInternal(map[string]any{"update": err.Error()})
	}
	// 3) если ничего не обновили — либо конфликт версий, либо не найдено
	if ct.RowsAffected() == 0 {
		return models.NewConflict(map[string]any{"version": "mismatch or not found"})
	}
	return nil
}
