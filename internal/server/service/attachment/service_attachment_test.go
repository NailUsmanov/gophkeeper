package attachment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"go.uber.org/zap"
)

type fakeRepo struct {
	insertFn  func(ctx context.Context, meta *models.AttachmentMeta) error
	getByIDFn func(ctx context.Context, attachmentID string) (*models.AttachmentMeta, error)
	deleteFn  func(ctx context.Context, attachmentID string) error
	listFn    func(ctx context.Context, ownerID, secretID string, limit, offset int) ([]models.AttachmentMeta, int, error)
	pingFn    func(ctx context.Context) error

	// трекинг вызовов
	insertCalls int
	deletedIDs  []string

	// для проверки аргументов List
	lastListOwnerID  string
	lastListSecretID string
	lastListLimit    int
	lastListOffset   int
}

func (f *fakeRepo) Insert(ctx context.Context, meta *models.AttachmentMeta) error {
	f.insertCalls++
	if f.insertFn != nil {
		return f.insertFn(ctx, meta)
	}
	return nil
}
func (f *fakeRepo) GetByID(ctx context.Context, attachmentID string) (*models.AttachmentMeta, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, attachmentID)
	}
	return nil, errors.New("not implemented")
}
func (f *fakeRepo) Delete(ctx context.Context, attachmentID string) error {
	f.deletedIDs = append(f.deletedIDs, attachmentID)
	if f.deleteFn != nil {
		return f.deleteFn(ctx, attachmentID)
	}
	return nil
}
func (f *fakeRepo) List(ctx context.Context, ownerID, secretID string, limit, offset int) ([]models.AttachmentMeta, int, error) {
	f.lastListOwnerID = ownerID
	f.lastListSecretID = secretID
	f.lastListLimit = limit
	f.lastListOffset = offset
	if f.listFn != nil {
		return f.listFn(ctx, ownerID, secretID, limit, offset)
	}
	return nil, 0, nil
}
func (f *fakeRepo) Ping(ctx context.Context) error {
	if f.pingFn != nil {
		return f.pingFn(ctx)
	}
	return nil
}

type fakeStorage struct {
	saveFn   func(ctx context.Context, attachmentID string, r io.Reader) (int64, error)
	openFn   func(ctx context.Context, attachmentID string) (io.ReadCloser, error)
	deleteFn func(ctx context.Context, attachmentID string) error
	pingFn   func(ctx context.Context) error

	// Трекинг
	saveCalls   int
	openCalls   int
	deleteCalls int

	lastSavedID   string
	lastOpenedID  string
	lastDeletedID string
}

func (s *fakeStorage) Save(ctx context.Context, attachmentID string, r io.Reader) (int64, error) {
	s.saveCalls++
	s.lastSavedID = attachmentID
	if s.saveFn != nil {
		return s.saveFn(ctx, attachmentID, r)
	}
	// по умолчанию — читаем все в /dev/null, чтобы LimitedReader отработал
	n, err := io.Copy(io.Discard, r)
	return int64(n), err
}
func (s *fakeStorage) Open(ctx context.Context, attachmentID string) (io.ReadCloser, error) {
	s.openCalls++
	s.lastOpenedID = attachmentID
	if s.openFn != nil {
		return s.openFn(ctx, attachmentID)
	}
	return io.NopCloser(bytes.NewReader(nil)), nil
}
func (s *fakeStorage) Delete(ctx context.Context, attachmentID string) error {
	s.deleteCalls++
	s.lastDeletedID = attachmentID
	if s.deleteFn != nil {
		return s.deleteFn(ctx, attachmentID)
	}
	return nil
}
func (s *fakeStorage) Ping(ctx context.Context) error {
	if s.pingFn != nil {
		return s.pingFn(ctx)
	}
	return nil
}

// Test Upload.
func TestAttachmentService_Upload_OK(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	repo := &fakeRepo{}
	st := &fakeStorage{}

	svc := NewAttachmentService(repo, st, logger)

	// Данные «файла» 10 байт
	body := bytes.NewBufferString("0123456789")

	meta, err := svc.Upload(ctx, "u1", "s1", "a.txt", "text/plain", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta == nil {
		t.Fatalf("meta is nil")
	}
	if meta.ID == "" {
		t.Errorf("meta.ID must be non-empty")
	}
	if meta.OwnerID != "u1" || meta.SecretID != "s1" || meta.FileName != "a.txt" {
		t.Errorf("wrong meta fields: %+v", *meta)
	}
	if meta.Size != 10 {
		t.Errorf("wrong size: got %d want 10", meta.Size)
	}
	// ensure storage.Save был вызван с тем же ID, что и meta.ID
	if st.lastSavedID != meta.ID {
		t.Errorf("storage.Save id mismatch: got %s want %s", st.lastSavedID, meta.ID)
	}
}

func TestAttachmentService_Upload_Validation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()
	svc := NewAttachmentService(&fakeRepo{}, &fakeStorage{}, logger)

	_, err := svc.Upload(ctx, "", "s1", "f", "t", bytes.NewReader(nil))
	if err == nil {
		t.Errorf("expected error on empty ownerID")
	}
	_, err = svc.Upload(ctx, "u1", "", "f", "t", bytes.NewReader(nil))
	if err == nil {
		t.Errorf("expected error on empty secretID")
	}
	_, err = svc.Upload(ctx, "u1", "s1", "", "t", bytes.NewReader(nil))
	if err == nil {
		t.Errorf("expected error on empty filename")
	}
	_, err = svc.Upload(ctx, "u1", "s1", "f", "t", nil)
	if err == nil {
		t.Errorf("expected error on nil reader")
	}
}
func TestAttachmentService_Upload_TooLarge(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()
	st := &fakeStorage{}
	svc := NewAttachmentService(&fakeRepo{}, st, logger)

	// Сформируем поток > 20MiB (20MiB + 2 байта)
	const big = (20 << 20) + 2
	r := io.LimitReader(zeroReader{}, big) // бесконечные нули, но с лимитом > maxSize

	_, err := svc.Upload(ctx, "u1", "s1", "big.bin", "application/octet-stream", r)
	if err == nil {
		t.Fatalf("expected error for too large file")
	}
	// На превышении лимита сервис должен попытаться удалить блоб (best-effort)
	if st.deleteCalls == 0 {
		t.Errorf("expected storage.Delete to be called on too-large")
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
func TestAttachmentService_Upload_SaveError(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()
	st := &fakeStorage{
		saveFn: func(ctx context.Context, id string, r io.Reader) (int64, error) {
			return 0, errors.New("save failed")
		},
	}
	svc := NewAttachmentService(&fakeRepo{}, st, logger)

	_, err := svc.Upload(ctx, "u1", "s1", "a.txt", "text/plain", bytes.NewBufferString("data"))
	if err == nil {
		t.Fatalf("expected error on storage.Save failure")
	}
}

func TestAttachmentService_Upload_RepoInsertError_TriggersDelete(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	repo := &fakeRepo{
		insertFn: func(ctx context.Context, meta *models.AttachmentMeta) error {
			return errors.New("db down")
		},
	}
	st := &fakeStorage{}
	svc := NewAttachmentService(repo, st, logger)

	_, err := svc.Upload(ctx, "u1", "s1", "a.txt", "text/plain", bytes.NewBufferString("12345"))
	if err == nil {
		t.Fatalf("expected error on repo.Insert failure")
	}
	if st.deleteCalls != 1 {
		t.Fatalf("expected storage.Delete to be called once, got %d", st.deleteCalls)
	}
	// Проверим, что удаляли именно тот самый id, который сохраняли
	if st.lastDeletedID == "" || st.lastDeletedID != st.lastSavedID {
		t.Errorf("deleted id mismatch: saved=%s deleted=%s", st.lastSavedID, st.lastDeletedID)
	}
}

// Test Download.
func TestAttachmentService_Download_OK(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	// мета из репозитория
	repo := &fakeRepo{
		getByIDFn: func(ctx context.Context, id string) (*models.AttachmentMeta, error) {
			return &models.AttachmentMeta{
				ID:        id,
				OwnerID:   "u1",
				SecretID:  "s1",
				FileName:  "a.txt",
				Size:      3,
				CreatedAt: time.Now().UTC(),
			}, nil
		},
	}
	// blob откроется
	st := &fakeStorage{
		openFn: func(ctx context.Context, id string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewBufferString("abc")), nil
		},
	}
	svc := NewAttachmentService(repo, st, logger)

	meta, r, err := svc.Download(ctx, "u1", "att-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta == nil || r == nil {
		t.Fatalf("meta or reader is nil")
	}
	b, _ := io.ReadAll(r)
	if string(b) != "abc" {
		t.Errorf("unexpected reader content: %q", string(b))
	}
}

func TestAttachmentService_Download_Validation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()
	svc := NewAttachmentService(&fakeRepo{}, &fakeStorage{}, logger)

	if _, _, err := svc.Download(ctx, "", "x"); err == nil {
		t.Errorf("expected error on empty ownerID")
	}
	if _, _, err := svc.Download(ctx, "u1", ""); err == nil {
		t.Errorf("expected error on empty attachmentID")
	}
}

func TestAttachmentService_Download_Forbidden(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	repo := &fakeRepo{
		getByIDFn: func(ctx context.Context, id string) (*models.AttachmentMeta, error) {
			return &models.AttachmentMeta{ID: id, OwnerID: "another"}, nil
		},
	}
	svc := NewAttachmentService(repo, &fakeStorage{}, logger)

	if _, _, err := svc.Download(ctx, "u1", "att-1"); err == nil {
		t.Fatalf("expected forbidden error")
	}
}

func TestAttachmentService_Download_OpenError_ReturnsMeta(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	retMeta := &models.AttachmentMeta{ID: "att-1", OwnerID: "u1", FileName: "a.txt"}
	repo := &fakeRepo{
		getByIDFn: func(ctx context.Context, id string) (*models.AttachmentMeta, error) {
			return retMeta, nil
		},
	}
	st := &fakeStorage{
		openFn: func(ctx context.Context, id string) (io.ReadCloser, error) {
			return nil, errors.New("open failed")
		},
	}
	svc := NewAttachmentService(repo, st, logger)

	meta, r, err := svc.Download(ctx, "u1", "att-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	if meta == nil || meta.ID != "att-1" {
		t.Fatalf("expected meta to be returned on open error")
	}
	if r != nil {
		t.Fatalf("expected nil reader on open error")
	}
}

// Test List.
func TestAttachmentService_List_OK(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	repo := &fakeRepo{
		listFn: func(ctx context.Context, ownerID, secretID string, limit, offset int) ([]models.AttachmentMeta, int, error) {
			return []models.AttachmentMeta{
				{ID: "a1", OwnerID: ownerID, SecretID: secretID, FileName: "1.txt"},
				{ID: "a2", OwnerID: ownerID, SecretID: secretID, FileName: "2.txt"},
			}, 2, nil
		},
	}
	svc := NewAttachmentService(repo, &fakeStorage{}, logger)

	items, total, err := svc.List(ctx, "u1", "s1", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("want total=2, items=2; got total=%d, items=%d", total, len(items))
	}
}

func TestAttachmentService_List_ValidationAndNormalization(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	repo := &fakeRepo{
		listFn: func(ctx context.Context, ownerID, secretID string, limit, offset int) ([]models.AttachmentMeta, int, error) {
			return nil, 0, nil
		},
	}
	svc := NewAttachmentService(repo, &fakeStorage{}, logger)

	// пустые owner/secret → ошибка
	if _, _, err := svc.List(ctx, "", "s1", 10, 0); err == nil {
		t.Errorf("expected error on empty ownerID")
	}
	if _, _, err := svc.List(ctx, "u1", "", 10, 0); err == nil {
		t.Errorf("expected error on empty secretID")
	}

	// limit<=0 → по контракту нормализуем к 20
	if _, _, err := svc.List(ctx, "u1", "s1", 0, 0); err != nil {
		t.Errorf("unexpected error on normalized limit: %v", err)
	}
	if repo.lastListLimit != 20 {
		t.Errorf("expected normalized limit=20, got %d", repo.lastListLimit)
	}

	// limit>100 → резать до 100
	if _, _, err := svc.List(ctx, "u1", "s1", 500, 0); err != nil {
		t.Errorf("unexpected error on normalized limit>100: %v", err)
	}
	if repo.lastListLimit != 100 {
		t.Errorf("expected normalized limit=100, got %d", repo.lastListLimit)
	}

	// offset<0 → 0
	if _, _, err := svc.List(ctx, "u1", "s1", 10, -5); err != nil {
		t.Errorf("unexpected error on normalized offset: %v", err)
	}
	if repo.lastListOffset != 0 {
		t.Errorf("expected normalized offset=0, got %d", repo.lastListOffset)
	}
}

func TestAttachmentService_List_RepoError(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	repo := &fakeRepo{
		listFn: func(ctx context.Context, ownerID, secretID string, limit, offset int) ([]models.AttachmentMeta, int, error) {
			return nil, 0, errors.New("db err")
		},
	}
	svc := NewAttachmentService(repo, &fakeStorage{}, logger)

	if _, _, err := svc.List(ctx, "u1", "s1", 10, 0); err == nil {
		t.Fatalf("expected error on repo.List")
	}
}
