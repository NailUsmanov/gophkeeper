// Package postgres используется для связи с БД.
//
// auth_repo.go создает, проверяет, находит данные пользователя в БД.
package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/storage/postgres"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// UserRepository структура для инициализации БД.
type UserRepository struct {
	db postgres.PgxPool
}

// NewUserRepository конструктор для UserRepository.
func NewUserRepository(db postgres.PgxPool) *UserRepository {
	return &UserRepository{db: db}
}

// Create - добавляет информацию по новому юзеру в БД.
// Ожидается, что u.ID уже сгенерирован в сервисе (uuid.New().String()).
func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	// 1. Валидация минимально необходимых полей
	if u.ID == "" || u.Email == "" || u.PasswordHash == "" {
		return models.NewValidation(map[string]any{"fields": "id/email/password_hash required"})
	}
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	ct, err := r.db.Exec(ctx, postgres.CreateUserQuery,
		u.ID,
		u.Email,
		u.CreatedAt.UTC(),
		u.PasswordHash)
	if err != nil {
		// проверяем нет ли уже такого email в базе.
		// Postgres возвращает ошибку, приводим ее к pgconn.PgError,
		// сравниваем ее код с pgerrcode.UniqueViolation(23505)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.UniqueViolation {
			switch pgErr.ConstraintName {
			case "ux_users_email": // ← должно совпадать с миграцией
				return models.NewConflict(map[string]any{"email": "already exists"})
			case "users_pkey":
				return models.NewConflict(map[string]any{"id": "duplicate"})
			default:
				return models.NewConflict(nil)
			}
		}
		return models.NewInternal(map[string]any{"db": "insert failed"})
	}
	// 2. Гарантируем, что вставлена 1 строка
	if ct.RowsAffected() != 1 {
		return models.NewInternal(map[string]any{"insert": "no rows affected"})
	}
	return nil
}

// FindUserByEmail ищет пользователя в БД по email.
func (r *UserRepository) FindUserByEmail(ctx context.Context, email string) (*models.User, error) {
	// 1. Приведем email к правильному виду.
	email = strings.ToLower(strings.TrimSpace(email))
	// 2. Локальные переменные для чтения «сырых» значений из БД.
	var (
		id, dbEmail, passHash string
		createAt              time.Time
	)
	// 3. Делаем SELECT из БД.
	err := r.db.QueryRow(ctx, postgres.FindUserByEmailQuery, email).Scan(&id, &dbEmail, &passHash, &createAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.NewNotFound(map[string]any{"email": email})
		}
		return nil, models.NewInternal(map[string]any{"select": err.Error()})
	}
	// 4. Собираем доменную модель пользователя и возвращаем ее.
	user := models.User{
		ID:           id,
		Email:        dbEmail,
		PasswordHash: passHash,
		CreatedAt:    createAt.UTC(),
	}
	return &user, nil
}

// FindUserByID находит пользователя из БД по ID.
func (r *UserRepository) FindUserByID(ctx context.Context, userID string) (*models.User, error) {
	// 1. Локальные переменные для чтения сырых значений из БД.
	var (
		dbID, email, passHash string
		createdAt             time.Time
	)
	// 2. Делаем SELECT из БД.
	err := r.db.QueryRow(ctx, postgres.FindUserByIDQuery, userID).Scan(&dbID, &email, &passHash, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.NewNotFound(map[string]any{"id": userID})
		}
		return nil, models.NewInternal(map[string]any{"select": err.Error()})
	}
	// 3. Собираем доменную модель и возвращаем ее.
	user := models.User{
		ID:           dbID,
		Email:        email,
		PasswordHash: passHash,
		CreatedAt:    createdAt.UTC(),
	}
	return &user, nil
}
