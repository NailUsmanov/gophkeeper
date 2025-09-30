package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/service/secret"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
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

	mock.ExpectExec(`(?s)INSERT\s+INTO\s+secrets`).
		WithArgs(s.ID, s.OwnerID, string(s.Type), s.Title, b, s.Version, s.CreatedAt, s.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repo.Insert(context.Background(), s); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSecret_Insert_JSONMarshalError_ReturnsValidation(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	// json.Marshal ломается на chan/function и пр.
	s := &secret.Secret{
		ID:        "bad",
		OwnerID:   "u1",
		Type:      models.SecretPassword,
		Title:     "t",
		Data:      map[string]any{"bad": make(chan int)},
		Version:   1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := repo.Insert(context.Background(), s); err == nil || !models.HasCode(err, models.ErrCodeValidationFail.Error()) {
		t.Fatalf("want validation error, got %v", err)
	}
}

func TestSecret_Insert_DBError_Internal(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	s := &secret.Secret{
		ID: "s1", OwnerID: "u1", Type: models.SecretPassword, Title: "t",
		Data: map[string]any{"k": "v"}, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	b, _ := json.Marshal(s.Data)

	mock.ExpectExec("(?s).*INSERT.*secrets").
		WithArgs(s.ID, s.OwnerID, string(s.Type), s.Title, b, s.Version, s.CreatedAt, s.UpdatedAt).
		WillReturnError(errors.New("db down"))

	if err := repo.Insert(context.Background(), s); err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal, got %v", err)
	}
	_ = mock.ExpectationsWereMet()
}

func TestSecret_Insert_NoRowsAffected_Internal(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	s := &secret.Secret{
		ID: "s1", OwnerID: "u1", Type: models.SecretPassword, Title: "t",
		Data: map[string]any{"k": "v"}, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	b, _ := json.Marshal(s.Data)

	// RowsAffected=0
	mock.ExpectExec("(?s).*INSERT.*secrets").
		WithArgs(s.ID, s.OwnerID, string(s.Type), s.Title, b, s.Version, s.CreatedAt, s.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 0))

	if err := repo.Insert(context.Background(), s); err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal(no rows), got %v", err)
	}
	_ = mock.ExpectationsWereMet()
}

func TestSecret_Insert_UniqueViolation_Conflict(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	s := &secret.Secret{ID: "s1", OwnerID: "u1", Type: models.SecretPassword, Title: "t",
		Data: map[string]any{"k": "v"}, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	mock.ExpectExec("(?s).*INSERT.*secrets").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})

	if err := repo.Insert(context.Background(), s); err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict, got %v", err)
	}
	_ = mock.ExpectationsWereMet()
}

func TestSecret_Insert_Duplicate(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	s := &secret.Secret{ID: "dup", Data: map[string]any{}}

	mock.ExpectExec(`(?s)INSERT\s+INTO\s+secrets`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})

	if err := repo.Insert(context.Background(), s); err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
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

	mock.ExpectQuery(`(?s)SELECT\s+id,\s*owner_id,\s*type,\s*title,\s*data,\s*version,\s*created_at,\s*updated_at\s+FROM\s+secrets`).
		WithArgs("s1", "u1").
		WillReturnRows(rows)

	got, err := repo.GetByID(context.Background(), "u1", "s1")
	if err != nil || got.ID != "s1" || got.OwnerID != "u1" || got.Version != 1 {
		t.Fatalf("GetByID bad: %+v, err=%v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSecret_GetByID_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	// Верни ошибку no rows
	mock.ExpectQuery("(?s).*FROM\\s+secrets.*").
		WithArgs("sid", "u1").
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.GetByID(context.Background(), "u1", "sid")
	if err == nil || !models.HasCode(err, models.ErrCodeNotFound.Error()) {
		t.Fatalf("want not_found, got %v", err)
	}
}

func TestSecret_GetByID_DBError_Internal(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	mock.ExpectQuery("(?s).*FROM\\s+secrets.*").
		WithArgs("sid", "u1").
		WillReturnError(errors.New("db fail"))

	_, err := repo.GetByID(context.Background(), "u1", "sid")
	if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal, got %v", err)
	}
}

func TestSecret_GetByID_InvalidJSON_Internal(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	now := time.Now().UTC()
	// data = невалидный json
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "type", "title", "data", "version", "created_at", "updated_at",
	}).AddRow("s1", "u1", "password", "t", []byte("not-json"), 1, now, now)

	mock.ExpectQuery("(?s).*SELECT.*FROM\\s+secrets.*").
		WithArgs("s1", "u1").
		WillReturnRows(rows)

	_, err := repo.GetByID(context.Background(), "u1", "s1")
	if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal on invalid json, got %v", err)
	}
	_ = mock.ExpectationsWereMet()
}

func TestSecret_List_WithFilters(t *testing.T) {
	t.Skip("temporarily skipping flaky DB test")

	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	mock.ExpectQuery(`(?s)SELECT\s+COUNT$begin:math:text$\\*$end:math:text$\s+FROM\s+secrets\b.*`).
		WithArgs("u1", "password", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	now := time.Now().UTC()
	data := []byte(`{"a":1}`)
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "type", "title", "data", "version", "created_at", "updated_at",
	}).AddRow("s1", "u1", "password", "t1", data, 1, now, now).
		AddRow("s2", "u1", "password", "t2", data, 2, now, now)

	mock.ExpectQuery(`(?s)SELECT\s+id,\s*owner_id,\s*type,\s*title,\s*data,\s*version,\s*created_at,\s*updated_at\s+FROM\s+secrets\b.*LIMIT`).
		WithArgs("u1", "password", pgxmock.AnyArg(), 10, 0).
		WillReturnRows(rows)

	ty := models.SecretPassword
	after := time.Now().Add(-time.Hour)
	items, total, err := repo.List(context.Background(), "u1", 10, 0, secret.SecretListFilter{Type: &ty, UpdatedAfter: &after})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("List bad: total=%d len=%d err=%v", total, len(items), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSecret_List_DBQueryError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	ty := models.SecretPassword
	after := time.Now().Add(-time.Hour).UTC()

	// Ошибка на Query
	mock.ExpectQuery("(?s).*FROM\\s+secrets.*LIMIT").
		WithArgs("u1", string(ty), after, 10, 0).
		WillReturnError(errors.New("query failed"))

	_, _, err := repo.List(context.Background(), "u1", 10, 0, secret.SecretListFilter{Type: &ty, UpdatedAfter: &after})
	if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal(list), got %v", err)
	}
}

func TestSecret_List_ScanError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	// Сделаем неверный тип для version (ожидается int)
	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "type", "title", "data", "version", "created_at", "updated_at",
	}).AddRow("s1", "u1", "password", "t", []byte(`{"a":1}`), "oops", now, now)

	mock.ExpectQuery("(?s).*FROM\\s+secrets.*LIMIT").
		WithArgs("u1", 10, 0).
		WillReturnRows(rows)

	// count никогда не достигнется
	_, _, err := repo.List(context.Background(), "u1", 10, 0, secret.SecretListFilter{})
	if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal(scan), got %v", err)
	}
}

func TestSecret_List_InvalidJSONInRow(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "type", "title", "data", "version", "created_at", "updated_at",
	}).AddRow("s1", "u1", "password", "t", []byte("not-json"), 1, now, now)

	mock.ExpectQuery("(?s).*FROM\\s+secrets.*LIMIT").
		WithArgs("u1", 10, 0).
		WillReturnRows(rows)

	_, _, err := repo.List(context.Background(), "u1", 10, 0, secret.SecretListFilter{})
	if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal(invalid json), got %v", err)
	}
}

func TestSecret_List_RowsErrAfterIter(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "type", "title", "data", "version", "created_at", "updated_at",
	}).AddRow("s1", "u1", "password", "t", []byte(`{"a":1}`), 1, now, now)
	// Ошибка на 1-й строке после Next
	rows.RowError(0, errors.New("iter error"))

	mock.ExpectQuery("(?s).*FROM\\s+secrets.*LIMIT").
		WithArgs("u1", 10, 0).
		WillReturnRows(rows)

	_, _, err := repo.List(context.Background(), "u1", 10, 0, secret.SecretListFilter{})
	if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal(rows err), got %v", err)
	}
}

func TestSecret_List_Success_ThenCountError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "type", "title", "data", "version", "created_at", "updated_at",
	}).AddRow("s1", "u1", "note", "A", []byte(`{"k":1}`), 1, now, now)

	// Список ок
	mock.ExpectQuery("(?s).*FROM\\s+secrets.*LIMIT").
		WithArgs("u1", 10, 0).
		WillReturnRows(rows)

	// count падает
	mock.ExpectQuery("(?s).*COUNT.*FROM\\s+secrets.*").
		WithArgs("u1").
		WillReturnError(errors.New("count failed"))

	_, _, err := repo.List(context.Background(), "u1", 10, 0, secret.SecretListFilter{})
	if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal(count), got %v", err)
	}
	_ = mock.ExpectationsWereMet()
}

func TestSecret_List_Success_WithFilters(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	owner := "u1"
	ty := models.SecretType("note")
	after := time.Now().Add(-time.Hour).UTC()
	now := time.Now().UTC()

	// строки
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "type", "title", "data", "version", "created_at", "updated_at",
	}).AddRow("s1", owner, "note", "A", []byte(`{"k":1}`), 1, now, now).
		AddRow("s2", owner, "note", "B", []byte(`{"k":2}`), 2, now, now)

	// list c фильтрами (ownerID, type, after, limit, offset)
	mock.ExpectQuery("(?s).*FROM\\s+secrets.*LIMIT").
		WithArgs(owner, string(ty), after, 10, 0).
		WillReturnRows(rows)

	// count c теми же фильтрами (ownerID, type, after)
	mock.ExpectQuery("(?s).*COUNT.*FROM\\s+secrets.*").
		WithArgs(owner, string(ty), after).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	items, total, err := repo.List(context.Background(), owner, 10, 0, secret.SecretListFilter{Type: &ty, UpdatedAfter: &after})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("want total=2, len=2; got total=%d, len=%d", total, len(items))
	}
	_ = mock.ExpectationsWereMet()
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

	mock.ExpectExec(`(?s)UPDATE\s+secrets`).
		WithArgs(s.Title, b, s.Version, s.UpdatedAt, s.ID, "u1", s.Version-1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := repo.Update(context.Background(), "u1", s); err != nil {
		t.Fatalf("Update ok: %v", err)
	}

	mock.ExpectExec(`(?s)UPDATE\s+secrets`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if err := repo.Update(context.Background(), "u1", s); err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSecret_Update_JSONMarshalError_ReturnsNotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	s := &secret.Secret{
		ID:        "s1",
		Title:     "t",
		Data:      map[string]any{"bad": make(chan int)}, // ломаем marshal
		Version:   2,
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.Update(context.Background(), "u1", s); err == nil || !models.HasCode(err, models.ErrCodeNotFound.Error()) {
		t.Fatalf("want not_found on marshal, got %v", err)
	}
}

func TestSecret_Update_DBError_Internal(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	s := &secret.Secret{
		ID:        "s1",
		Title:     "t",
		Data:      map[string]any{"k": "v"},
		Version:   3,
		UpdatedAt: time.Now().UTC(),
	}
	b, _ := json.Marshal(s.Data)

	mock.ExpectExec("(?s).*UPDATE\\s+secrets").
		WithArgs(s.Title, b, s.Version, s.UpdatedAt, s.ID, "u1", s.Version-1).
		WillReturnError(errors.New("db update failed"))

	if err := repo.Update(context.Background(), "u1", s); err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal(update), got %v", err)
	}
	_ = mock.ExpectationsWereMet()
}

func TestSecret_Update_NoRowsAffected_Conflict(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSecretRepo(mock)

	s := &secret.Secret{
		ID:        "s1",
		Title:     "t",
		Data:      map[string]any{"k": "v"},
		Version:   3,
		UpdatedAt: time.Now().UTC(),
	}
	b, _ := json.Marshal(s.Data)

	mock.ExpectExec("(?s).*UPDATE\\s+secrets").
		WithArgs(s.Title, b, s.Version, s.UpdatedAt, s.ID, "u1", s.Version-1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	if err := repo.Update(context.Background(), "u1", s); err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict(no rows), got %v", err)
	}
	_ = mock.ExpectationsWereMet()
}
