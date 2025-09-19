package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pgxmock "github.com/pashagolub/pgxmock/v2"
)

func TestUser_Create_OK(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	u := &models.User{ID: "u1", Email: "Test@Mail.com", PasswordHash: "hash", CreatedAt: time.Now().UTC()}

	mock.ExpectExec("INSERT INTO users").
		WithArgs(u.ID, "test@mail.com", u.CreatedAt, u.PasswordHash). // email нормализуется
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestUser_Create_UniqueEmail(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	u := &models.User{ID: "u1", Email: "a@a", PasswordHash: "h", CreatedAt: time.Now().UTC()}

	mock.ExpectExec("INSERT INTO users").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: "ux_users_email"})

	if err := repo.Create(context.Background(), u); err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict by email, got %v", err)
	}
}

func TestUser_FindByEmail_OK(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at"}).
		AddRow("u1", "a@a", "h", now)

	mock.ExpectQuery("SELECT id,email,password_hash,created_at FROM users WHERE email =").
		WithArgs("a@a").
		WillReturnRows(rows)

	u, err := repo.FindUserByEmail(context.Background(), "A@A")
	if err != nil || u.ID != "u1" || u.Email != "a@a" {
		t.Fatalf("FindUserByEmail: %+v, err=%v", u, err)
	}
}

func TestUser_FindByID_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT id, email, password_hash, created_at FROM users WHERE id =").
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.FindUserByID(context.Background(), "missing")
	if err == nil || !models.HasCode(err, models.ErrCodeNotFound.Error()) {
		t.Fatalf("want not_found, got %v", err)
	}
}
