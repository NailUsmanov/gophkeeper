package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"go.uber.org/zap"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/middlewares"
)

// ---- вспомогалки ----

type pingSvcMock struct{ err error }

func (m pingSvcMock) Ping(_ context.Context) error { return m.err }

func testLogger(t *testing.T) *zap.SugaredLogger {
	l, _ := zap.NewDevelopment()
	t.Cleanup(func() { _ = l.Sync() })
	return l.Sugar()
}

// кладём userID в контекст, как это делает твой middleware
func withUser(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), middlewares.UserLoginKey, userID)
	return req.WithContext(ctx)
}

// лёгкая обёртка, чтобы не валиться на Close()
type nopReadCloser struct{ io.Reader }

func (nopReadCloser) Close() error { return nil }

// собираем multipart тело: поле secret_id + файл под ключом "file"
func buildMultipart(secretID, filename string, content []byte) (contentType string, body *bytes.Buffer, err error) {
	body = &bytes.Buffer{}
	w := multipart.NewWriter(body)

	if err = w.WriteField("secret_id", secretID); err != nil {
		return "", nil, err
	}
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", nil, err
	}
	if _, err = fw.Write(content); err != nil {
		return "", nil, err
	}
	if err = w.Close(); err != nil {
		return "", nil, err
	}
	return w.FormDataContentType(), body, nil
}

// лёгкая обёртка, чтобы не валиться на Close()

// минимальный ответ, чтобы распарсить JSON из хендлера
type attachmentResp struct {
	ID          string `json:"id"`
	SecretID    string `json:"secret_id"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	CreatedAt   string `json:"created_at"`
}

// ---- ТЕСТЫ: NewUpload ----

func TestNewUpload_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	// что вернёт сервис
	now := time.Now().UTC()
	fileBytes := []byte("hello world")
	expectedID := "att-1"

	svc.
		EXPECT().
		Upload(
			gomock.Any(), // ctx
			"u1",         // ownerID
			"s1",         // secretID
			"photo.jpg",  // fileName
			gomock.Any(), // contentType (в multipart он может быть пустой, поэтому не жёстко)
			gomock.Any(), // io.Reader
		).
		DoAndReturn(func(ctx context.Context, ownerID, secretID, fileName, contentType string, r io.Reader) (*models.AttachmentMeta, error) {
			// можно дополнительно проверить, что ридер читается
			b, _ := io.ReadAll(r)
			if !bytes.Equal(b, fileBytes) {
				t.Fatalf("reader content mismatch: %q", string(b))
			}
			return &models.AttachmentMeta{
				ID:          expectedID,
				SecretID:    secretID,
				FileName:    fileName,
				ContentType: contentType,
				Size:        int64(len(fileBytes)),
				CreatedAt:   now,
			}, nil
		})

	// собираем запрос
	ct, body, err := buildMultipart("s1", "photo.jpg", fileBytes)
	if err != nil {
		t.Fatalf("build multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/attachments", body)
	req.Header.Set("Content-Type", ct)
	req = withUser(req, "u1") // ВАЖНО: строка, а не int

	rr := httptest.NewRecorder()

	// роут и хендлер
	r := chi.NewRouter()
	r.Post("/api/attachments", NewUpload(svc, log))

	// выполняем
	r.ServeHTTP(rr, req)

	// проверки
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	// проверим JSON
	var got attachmentResp
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v, body=%s", err, rr.Body.String())
	}
	if got.ID != expectedID || got.SecretID != "s1" || got.FileName != "photo.jpg" || got.Size != int64(len(fileBytes)) {
		t.Fatalf("unexpected response: %+v", got)
	}

	// Location заголовок опционален, но если ставишь — проверим
	if loc := rr.Header().Get("Location"); loc != "/api/attachments/"+expectedID {
		t.Fatalf("Location = %q, want /api/attachments/%s", loc, expectedID)
	}
}

func TestNewUpload_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	ct, body, _ := buildMultipart("s1", "photo.jpg", []byte("x"))
	req := httptest.NewRequest(http.MethodPost, "/api/attachments", body)
	req.Header.Set("Content-Type", ct)
	// НЕ кладём user в контекст → 401

	rr := httptest.NewRecorder()
	r := chi.NewRouter()
	r.Post("/api/attachments", NewUpload(svc, log))
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestNewUpload_MissingSecretID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	// multipart без secret_id: сделаем только файл
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, _ := w.CreateFormFile("file", "photo.jpg")
	_, _ = fw.Write([]byte("x"))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/attachments", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = withUser(req, "u1")

	rr := httptest.NewRecorder()
	r := chi.NewRouter()
	r.Post("/api/attachments", NewUpload(svc, log))
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestNewUpload_MissingFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	// multipart без файла: только поле secret_id
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.WriteField("secret_id", "s1")
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/attachments", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = withUser(req, "u1")

	rr := httptest.NewRecorder()
	r := chi.NewRouter()
	r.Post("/api/attachments", NewUpload(svc, log))
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ---- ТЕСТЫ: NewDownload ----

func TestNewDownload_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	const (
		userID   = "u1"
		attID    = "att-1"
		filename = "file.txt"
		ct       = "text/plain"
	)
	data := []byte("hello download")

	svc.EXPECT().
		Download(gomock.Any(), userID, attID).
		DoAndReturn(func(ctx context.Context, ownerID, attachmentID string) (*models.AttachmentMeta, io.ReadCloser, error) {
			meta := &models.AttachmentMeta{
				ID:          attID,
				SecretID:    "s1",
				FileName:    filename,
				ContentType: ct,
				Size:        int64(len(data)),
				CreatedAt:   time.Now().UTC(),
			}
			return meta, nopReadCloser{bytes.NewReader(data)}, nil
		})

	// роутер с путём /api/attachments/{id}
	r := chi.NewRouter()
	r.Get("/api/attachments/{id}", NewDownload(svc, log))

	req := httptest.NewRequest(http.MethodGet, "/api/attachments/"+attID, nil)
	req = withUser(req, userID)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d, body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}
	// тело
	if !bytes.Equal(rr.Body.Bytes(), data) {
		t.Fatalf("body mismatch: %q", rr.Body.String())
	}
	// заголовки
	if got := rr.Header().Get("Content-Type"); got != ct {
		t.Fatalf("Content-Type=%q, want %q", got, ct)
	}
	if got := rr.Header().Get("Content-Length"); got != strconv.Itoa(len(data)) {
		t.Fatalf("Content-Length=%q, want %d", got, len(data))
	}
	if got := rr.Header().Get("Content-Disposition"); !strings.Contains(got, filename) {
		t.Fatalf("Content-Disposition=%q, want to contain filename %q", got, filename)
	}
}

func TestNewDownload_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	r := chi.NewRouter()
	r.Get("/api/attachments/{id}", NewDownload(svc, log))

	req := httptest.NewRequest(http.MethodGet, "/api/attachments/att-1", nil)
	// без пользователя
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestNewDownload_MissingID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	// вызываем хендлер напрямую, подложив пустой chi.RouteContext
	h := NewDownload(svc, log)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = withUser(req, "u1")

	rctx := chi.NewRouteContext()
	// id не добавляем, будет пустым
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestNewDownload_Forbidden_And_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := testLogger(t)

	// 403
	{
		svc := NewMockAttachmentService(ctrl)
		svc.EXPECT().
			Download(gomock.Any(), "u1", "att403").
			Return(nil, nil, models.NewForbidden(nil))

		r := chi.NewRouter()
		r.Get("/api/attachments/{id}", NewDownload(svc, log))

		req := httptest.NewRequest(http.MethodGet, "/api/attachments/att403", nil)
		req = withUser(req, "u1")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("status=%d, want %d", rr.Code, http.StatusForbidden)
		}
	}

	// 404
	{
		svc := NewMockAttachmentService(ctrl)
		svc.EXPECT().
			Download(gomock.Any(), "u1", "att404").
			Return(nil, nil, models.NewNotFound(nil))

		r := chi.NewRouter()
		r.Get("/api/attachments/{id}", NewDownload(svc, log))

		req := httptest.NewRequest(http.MethodGet, "/api/attachments/att404", nil)
		req = withUser(req, "u1")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status=%d, want %d", rr.Code, http.StatusNotFound)
		}
	}
}

// ---- ТЕСТЫ: NewListAttachments ----

type listResp struct {
	Items  []attachmentResp `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func TestNewListAttachments_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	items := []models.AttachmentMeta{
		{ID: "a1", SecretID: "s1", FileName: "f1.txt", ContentType: "text/plain", Size: 1, CreatedAt: time.Now().UTC()},
		{ID: "a2", SecretID: "s1", FileName: "f2.txt", ContentType: "text/plain", Size: 2, CreatedAt: time.Now().UTC()},
	}
	svc.EXPECT().
		List(gomock.Any(), "u1", "s1", 10, 0).
		Return(items, 2, nil)

	r := chi.NewRouter()
	r.Get("/api/attachments", NewListAttachments(svc, log))

	req := httptest.NewRequest(http.MethodGet, "/api/attachments?secret_id=s1&limit=10&offset=0", nil)
	req = withUser(req, "u1")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type=%q, want application/json", ct)
	}

	var got listResp
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v, body=%s", err, rr.Body.String())
	}
	if got.Total != 2 || got.Limit != 10 || got.Offset != 0 || len(got.Items) != 2 {
		t.Fatalf("unexpected pagination: %+v", got)
	}
	if got.Items[0].ID != "a1" || got.Items[1].ID != "a2" {
		t.Fatalf("unexpected items: %+v", got.Items)
	}
}

func TestNewListAttachments_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	r := chi.NewRouter()
	r.Get("/api/attachments", NewListAttachments(svc, log))

	req := httptest.NewRequest(http.MethodGet, "/api/attachments?secret_id=s1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestNewListAttachments_MissingSecretID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	r := chi.NewRouter()
	r.Get("/api/attachments", NewListAttachments(svc, log))

	req := httptest.NewRequest(http.MethodGet, "/api/attachments?limit=5", nil)
	req = withUser(req, "u1")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestNewListAttachments_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockAttachmentService(ctrl)
	log := testLogger(t)

	svc.EXPECT().
		List(gomock.Any(), "u1", "sX", 20, 0).
		Return(nil, 0, models.NewForbidden(nil))

	r := chi.NewRouter()
	r.Get("/api/attachments", NewListAttachments(svc, log))

	req := httptest.NewRequest(http.MethodGet, "/api/attachments?secret_id=sX", nil)
	req = withUser(req, "u1")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestPing_OK(t *testing.T) {
	log := zap.NewNop().Sugar()
	hdl := NewPing(pingSvcMock{err: nil}, log) // без h.

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	hdl(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestPing_Error(t *testing.T) {
	log := zap.NewNop().Sugar()
	hdl := NewPing(pingSvcMock{err: errors.New("db down")}, log)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	hdl(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", rr.Code)
	}
}
