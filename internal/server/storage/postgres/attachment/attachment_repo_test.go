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

func TestAttachment_Insert_OK(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := NewAttachmentRepository(mock)

	meta := &models.AttachmentMeta{
		ID: "a1", FileName: "f.txt", SecretID: "s1", OwnerID: "u1",
		Size: 3, ContentType: "text/plain", CreatedAt: time.Now().UTC(),
	}

	mock.ExpectExec("INSERT INTO attachments_meta").
		WithArgs(meta.ID, meta.FileName, meta.SecretID, meta.OwnerID, meta.Size, meta.ContentType, meta.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repo.Insert(context.Background(), meta); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAttachment_Insert_UniqueViolation(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAttachmentRepository(mock)

	meta := &models.AttachmentMeta{ID: "dup", CreatedAt: time.Now().UTC()}

	mock.ExpectExec("INSERT INTO attachments_meta").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})

	if err := repo.Insert(context.Background(), meta); err == nil || !models.HasCode(err, models.ErrCodeConflict.Error()) {
		t.Fatalf("want conflict, got %v", err)
	}
}

func TestAttachment_Insert_NoRowsAffected(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAttachmentRepository(mock)

	meta := &models.AttachmentMeta{ID: "a1", CreatedAt: time.Now().UTC()}

	mock.ExpectExec("INSERT INTO attachments_meta").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 0))

	err := repo.Insert(context.Background(), meta)
	if err == nil || !models.HasCode(err, models.ErrCodeInternal.Error()) {
		t.Fatalf("want internal on 0 rows, got %v", err)
	}
}

func TestAttachment_GetByID_OK(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAttachmentRepository(mock)

	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{
		"id", "file_name", "secret_id", "owner_id", "size", "content_type", "created_at",
	}).AddRow("a1", "f.txt", "s1", "u1", int64(3), "text/plain", now)

	mock.ExpectQuery("SELECT .* FROM attachments_meta").
		WithArgs("a1").
		WillReturnRows(rows)

	meta, err := repo.GetByID(context.Background(), "a1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if meta.ID != "a1" || meta.OwnerID != "u1" || meta.Size != 3 {
		t.Fatalf("bad meta: %+v", meta)
	}
}

func TestAttachment_GetByID_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAttachmentRepository(mock)

	mock.ExpectQuery("SELECT .* FROM attachments_meta").
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.GetByID(context.Background(), "missing")
	if err == nil || !models.HasCode(err, models.ErrCodeNotFound.Error()) {
		t.Fatalf("want not_found, got %v", err)
	}
}

func TestAttachment_Delete_OK_and_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAttachmentRepository(mock)

	// OK
	mock.ExpectExec("DELETE FROM attachments_meta").
		WithArgs("a1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := repo.Delete(context.Background(), "a1"); err != nil {
		t.Fatalf("Delete ok: %v", err)
	}

	// Not found
	mock.ExpectExec("DELETE FROM attachments_meta").
		WithArgs("nope").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	if err := repo.Delete(context.Background(), "nope"); err == nil || !models.HasCode(err, models.ErrCodeNotFound.Error()) {
		t.Fatalf("want not_found, got %v", err)
	}
}

func TestAttachment_List_OK(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAttachmentRepository(mock)

	// count
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM attachments_meta").
		WithArgs("u1", "s1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	// list
	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{
		"id", "file_name", "secret_id", "owner_id", "size", "content_type", "created_at",
	}).AddRow("a1", "f1", "s1", "u1", int64(1), "t1", now).
		AddRow("a2", "f2", "s1", "u1", int64(2), "t2", now)

	mock.ExpectQuery("SELECT id, file_name, secret_id, owner_id, size, content_type, created_at FROM attachments_meta").
		WithArgs("s1", "u1", 10, 0).
		WillReturnRows(rows)

	items, total, err := repo.List(context.Background(), "u1", "s1", 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 || len(items) != 2 || items[0].ID != "a1" {
		t.Fatalf("bad result: total=%d items=%d", total, len(items))
	}
}

func TestAttachment_Ping(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAttachmentRepository(mock)

	mock.ExpectPing()

	if err := repo.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := repo.Ping(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context canceled, got %v", err)
	}
}
