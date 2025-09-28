package postgres

import (
	"context"
	"errors"
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

	mock.ExpectExec(`(?s)INSERT\s+INTO\s+users`).
		WithArgs(u.ID, "test@mail.com", u.CreatedAt, u.PasswordHash). // email нормализуется
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUser_Create_ValidationError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	// пропущены поля
	u := &models.User{ID: "", Email: "", PasswordHash: ""}

	err := repo.Create(context.Background(), u)
	if err == nil || !models.HasCode(err, models.ErrCodeValidationFail.Error()) {
		t.Fatalf("want validation error, got %v", err)
	}
	// никаких ожиданий к БД не должно быть
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUser_Create_UniqueEmail(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	u := &models.User{ID: "u1", Email: "a@a", PasswordHash: "h", CreatedAt: time.Now().UTC()}

	mock.ExpectExec(`(?s)INSERT\s+INTO\s+users`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: "ux_users_email"})

	if err := repo.Create(context.Background(), u); err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict by email, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUser_Create_UniqueID_PKey(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	u := &models.User{ID: "u1", Email: "a@a", PasswordHash: "h", CreatedAt: time.Now().UTC()}

	mock.ExpectExec(`(?s)INSERT\s+INTO\s+users`).
		WithArgs(u.ID, "a@a", u.CreatedAt, u.PasswordHash).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: "users_pkey"})

	err := repo.Create(context.Background(), u)
	if err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict by id (pkey), got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUser_Create_InternalDBErrorAndZeroRows(t *testing.T) {
	t.Run("internal db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		repo := NewUserRepository(mock)

		u := &models.User{ID: "u1", Email: "a@a", PasswordHash: "h", CreatedAt: time.Now().UTC()}

		mock.ExpectExec(`(?s)INSERT\s+INTO\s+users`).
			WithArgs(u.ID, "a@a", u.CreatedAt, u.PasswordHash).
			WillReturnError(errors.New("some db error"))

		err := repo.Create(context.Background(), u)
		if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
			t.Fatalf("want internal, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("zero rows affected -> internal", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		repo := NewUserRepository(mock)

		u := &models.User{ID: "u1", Email: "a@a", PasswordHash: "h", CreatedAt: time.Now().UTC()}

		mock.ExpectExec(`(?s)INSERT\s+INTO\s+users`).
			WithArgs(u.ID, "a@a", u.CreatedAt, u.PasswordHash).
			WillReturnResult(pgxmock.NewResult("INSERT", 0))

		err := repo.Create(context.Background(), u)
		if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
			t.Fatalf("want internal on 0 rows, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestUser_FindByEmail_OK(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at"}).
		AddRow("u1", "a@a", "h", now)

	mock.ExpectQuery(`(?s)SELECT\s+id,email,password_hash,created_at\s+FROM\s+users\s+WHERE\s+email\s*=`).
		WithArgs("a@a").
		WillReturnRows(rows)

	u, err := repo.FindUserByEmail(context.Background(), "A@A")
	if err != nil || u.ID != "u1" || u.Email != "a@a" {
		t.Fatalf("FindUserByEmail: %+v, err=%v", u, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUser_FindByEmail_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery(`(?s)SELECT\s+id,email,password_hash,created_at\s+FROM\s+users\s+WHERE\s+email\s*=`).
		WithArgs("missing@mail").
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.FindUserByEmail(context.Background(), "Missing@mail")
	if err == nil || !models.HasCode(err, models.ErrCodeNotFound.Error()) {
		t.Fatalf("want not_found, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUser_FindByEmail_InternalError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery(`(?s)SELECT\s+id,email,password_hash,created_at\s+FROM\s+users\s+WHERE\s+email\s*=`).
		WithArgs("x@y").
		WillReturnError(errors.New("db down"))

	_, err := repo.FindUserByEmail(context.Background(), "x@y")
	if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUser_FindByID_OK(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at"}).
		AddRow("u1", "u@mail", "hash", now)

	mock.ExpectQuery(`(?s)SELECT\s+id,\s*email,\s*password_hash,\s*created_at\s+FROM\s+users\s+WHERE\s+id\s*=`).
		WithArgs("u1").
		WillReturnRows(rows)

	u, err := repo.FindUserByID(context.Background(), "u1")
	if err != nil {
		t.Fatalf("FindUserByID err: %v", err)
	}
	if u.ID != "u1" || u.Email != "u@mail" || u.PasswordHash != "hash" {
		t.Fatalf("bad user: %+v", u)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUser_FindByID_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery(`(?s)SELECT\s+id,\s*email,\s*password_hash,\s*created_at\s+FROM\s+users\s+WHERE\s+id\s*=`).
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.FindUserByID(context.Background(), "missing")
	if err == nil || !models.HasCode(err, models.ErrCodeNotFound.Error()) {
		t.Fatalf("want not_found, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUser_FindByID_InternalError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery(`(?s)SELECT\s+id,\s*email,\s*password_hash,\s*created_at\s+FROM\s+users\s+WHERE\s+id\s*=`).
		WithArgs("u1").
		WillReturnError(errors.New("driver error"))

	_, err := repo.FindUserByID(context.Background(), "u1")
	if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestUser_Create_UniqueUnknownConstraint_ConflictNoDetails(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	u := &models.User{ID: "u1", Email: "a@a", PasswordHash: "h", CreatedAt: time.Now().UTC()}

	// Вернём UniqueViolation c "странным" именем ограничения
	mock.ExpectExec(`(?s)INSERT\s+INTO\s+users`).
		WithArgs(u.ID, "a@a", u.CreatedAt, u.PasswordHash).
		WillReturnError(&pgconn.PgError{
			Code:           pgerrcode.UniqueViolation,
			ConstraintName: "some_weird_constraint",
		})

	err := repo.Create(context.Background(), u)
	if err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict, got %v", err)
	}
	// Проверим, что деталей нет (nil), как в default ветке switch
	var app *models.AppError
	if !errors.As(err, &app) {
		t.Fatalf("want *models.AppError, got %T", err)
	}
	if app.Details != nil && len(app.Details) != 0 {
		t.Fatalf("want nil/empty details, got %#v", app.Details)
	}

	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestUser_FindByEmail_NormalizesSpacesAndCase(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at"}).
		AddRow("u2", "b@b", "hash2", now)

	// Репо обязано подать в запрос ОЧИЩЕННУЮ и приведённую к нижнему регистру почту — "b@b"
	mock.ExpectQuery(`(?s)SELECT\s+id,email,password_hash,created_at\s+FROM\s+users\s+WHERE\s+email\s*=`).
		WithArgs("b@b").
		WillReturnRows(rows)

	u, err := repo.FindUserByEmail(context.Background(), "   B@B   ")
	if err != nil {
		t.Fatalf("FindUserByEmail err: %v", err)
	}
	if u.ID != "u2" || u.Email != "b@b" {
		t.Fatalf("bad user: %#v", u)
	}
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestUser_FindByID_ConvertsCreatedAtToUTC(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	// Сдадим время в нестандартной зоне (UTC+3), репо должно вернуть .UTC()
	loc := time.FixedZone("UTC+3", 3*3600)
	src := time.Date(2025, 1, 2, 3, 4, 5, 6, loc)
	wantUTC := src.UTC()

	rows := pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at"}).
		AddRow("u3", "c@c", "hash3", src)

	mock.ExpectQuery(`(?s)SELECT\s+id,\s*email,\s*password_hash,\s*created_at\s+FROM\s+users\s+WHERE\s+id\s*=`).
		WithArgs("u3").
		WillReturnRows(rows)

	u, err := repo.FindUserByID(context.Background(), "u3")
	if err != nil {
		t.Fatalf("FindUserByID err: %v", err)
	}
	if !u.CreatedAt.Equal(wantUTC) || u.CreatedAt.Location() != time.UTC {
		t.Fatalf("created_at not UTC: got %v (loc=%v), want %v (UTC)",
			u.CreatedAt, u.CreatedAt.Location(), wantUTC)
	}
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
