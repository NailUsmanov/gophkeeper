package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/service/secret"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	pgxmock "github.com/pashagolub/pgxmock/v2"
)

func TestSecret_Insert_OK(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	s := &secret.Secret{
		ID: "s1", OwnerID: "u1", Type: models.SecretPassword, Title: "t",
		Data: map[string]any{"k": "v"}, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	b, _ := json.Marshal(s.Data)

	mock.ExpectExec("INSERT INTO secrets").
		WithArgs(s.ID, s.OwnerID, string(s.Type), s.Title, b, s.Version, s.CreatedAt, s.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repo.Insert(context.Background(), s); err != nil {
		t.Fatalf("Insert: %v", err)
	}
}

func TestSecret_Insert_Duplicate(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	s := &secret.Secret{ID: "dup", Data: map[string]any{}}

	mock.ExpectExec("INSERT INTO secrets").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})

	if err := repo.Insert(context.Background(), s); err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict, got %v", err)
	}
}

func TestSecret_GetByID_OK(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	now := time.Now().UTC()
	data := []byte(`{"k":"v"}`)
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "type", "title", "data", "version", "created_at", "updated_at",
	}).AddRow("s1", "u1", "password", "t", data, 1, now, now)

	mock.ExpectQuery("SELECT id, owner_id, type, title, data, version, created_at, updated_at FROM secrets").
		WithArgs("s1", "u1").
		WillReturnRows(rows)

	got, err := repo.GetByID(context.Background(), "u1", "s1")
	if err != nil || got.ID != "s1" || got.OwnerID != "u1" || got.Version != 1 {
		t.Fatalf("GetByID bad: %+v, err=%v", got, err)
	}
}

func TestSecret_List_WithFilters(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	// count
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM secrets").
		WithArgs("u1", "password", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	// list
	now := time.Now().UTC()
	data := []byte(`{"a":1}`)
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "type", "title", "data", "version", "created_at", "updated_at",
	}).AddRow("s1", "u1", "password", "t1", data, 1, now, now).
		AddRow("s2", "u1", "password", "t2", data, 2, now, now)

	mock.ExpectQuery("SELECT id, owner_id, type, title, data, version, created_at,updated_at FROM secrets").
		WithArgs("u1", "password", pgxmock.AnyArg(), 10, 0).
		WillReturnRows(rows)

	ty := models.SecretPassword
	after := time.Now().Add(-time.Hour)
	items, total, err := repo.List(context.Background(), "u1", 10, 0, secret.SecretListFilter{Type: &ty, UpdatedAfter: &after})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("List bad: total=%d len=%d err=%v", total, len(items), err)
	}
}

func TestSecret_Update_OK_and_Conflict(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	s := &secret.Secret{
		ID: "s1", Title: "new", Data: map[string]any{"k": "v"}, Version: 2,
		UpdatedAt: time.Now().UTC(),
	}
	b, _ := json.Marshal(s.Data)

	// OK (RowsAffected=1)
	mock.ExpectExec("UPDATE secrets").
		WithArgs(s.Title, b, s.Version, s.UpdatedAt, s.ID, "u1", s.Version-1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := repo.Update(context.Background(), "u1", s); err != nil {
		t.Fatalf("Update ok: %v", err)
	}

	// Conflict (RowsAffected=0)
	mock.ExpectExec("UPDATE secrets").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if err := repo.Update(context.Background(), "u1", s); err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict, got %v", err)
	}
}
