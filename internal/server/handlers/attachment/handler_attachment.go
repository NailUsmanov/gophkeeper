// Package handlers описывает HTTP-хендлеры.
//
// handler_attachment - загрузка и скачивание файла/ов.
//
//go:generate mockgen -source=handler_attachment.go -destination=handler_attachment_mock_test.go -package=handlers AttachmentService
package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/middlewares"
	thttp "github.com/NailUsmanov/gophkeeper/internal/server/transport/http"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// AttachmentService — порт для веб-слоя (хендлеров).
// Хендлеры знают только об этой абстракции.
type AttachmentService interface {
	// Upload — принять файл для секрета owner'a.
	// r — поток байтов (из http.Request.Body или multipart.File). Возвращаем метаданные.
	Upload(ctx context.Context, ownerID, secretID, fileName, contentType string, r io.Reader) (*models.AttachmentMeta, error)
	// Download — отдать файл, если владелец совпадает.
	// Возвращаем метаданные и поток для чтения. Вызывающий обязан закрыть reader.
	Download(ctx context.Context, ownerID, attachmentID string) (meta *models.AttachmentMeta, reader io.ReadCloser, err error)
	List(ctx context.Context, ownerID, secretID string, limit, offset int) ([]models.AttachmentMeta, int, error)
	Ping(ctx context.Context) error
}

// maxUploadMB — жёсткий лимит тела запроса для upload на уровне HTTP.
// Даже если сервис откажет по размеру, это защитит от лишней прокачки в память/диск.
// maxFormMemMB — сколько multipart хранить в ОЗУ до сброса во временный файл.
const (
	maxUploadBytes  = 25 * 1024 * 1024 // 25 MB — лимит тела запроса на upload
	maxFormMemBytes = 32 * 1024 * 1024 // 32 MB — сколько multipart держим в RAM до tmp-файлов
)

// NewUpload обрабатывает POST /api/attachments (multipart/form-data с полем "file").
// secret_id берём из query (?secret_id=...) или из поля формы "secret_id".
func NewUpload(svc AttachmentService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Достаем ownerID
		ownerID, _ := r.Context().Value(middlewares.UserLoginKey).(string)
		if ownerID == "" {
			thttp.WriteError(w, models.NewUnauthorized(nil))
			return
		}
		// 2. Локальный лимит тела запроса — защита на уровне HTTP.
		if maxUploadBytes > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		}
		// 3. Разбор multipart: часть содержимого уйдёт во временные файлы, если превысит maxFormMemMB.
		if err := r.ParseMultipartForm(maxFormMemBytes); err != nil {
			thttp.WriteError(w, models.NewBadRequest(map[string]any{"op": "too big file"}))
			return
		}
		defer func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll() // удалит временные файлы
			}
		}()
		// 4. Достаём secret_id (сначала из формы, затем из query).
		secretID := r.FormValue("secret_id")
		if secretID == "" {
			secretID = r.URL.Query().Get("secret_id")
		}
		if secretID == "" {
			thttp.WriteError(w, models.NewBadRequest(map[string]any{"op": "empty secretID"}))
			return
		}
		// 4) Достаём файл (поле "file")
		file, header, err := r.FormFile("file")
		if err != nil {
			thttp.WriteError(w, models.NewBadRequest(map[string]any{"op": "required file"}))
			return
		}
		defer file.Close()

		fileName := header.Filename
		contentType := header.Header.Get("Content-Type")

		start := time.Now()
		meta, err := svc.Upload(r.Context(), ownerID, secretID, fileName, contentType, file)
		if err != nil {
			thttp.WriteError(w, err)
			return
		}
		// 5. Готовим DTO ответ.
		resp := toAttachmentResponse(*meta)

		// 6. Ответ
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/api/attachments/"+resp.ID)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)

		log.Infow("attachment upload ok",
			"user", ownerID, "secret", secretID, "att", resp.ID,
			"size", resp.Size, "mime", resp.ContentType, "took", time.Since(start),
		)

	}
}

// NewDownload обрабатывает GET /api/attachments/{id} .
func NewDownload(svc AttachmentService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Достаем ownerID из контекста.
		ownerID, _ := r.Context().Value(middlewares.UserLoginKey).(string)
		if ownerID == "" {
			thttp.WriteError(w, models.NewUnauthorized(nil))
			return
		}
		// 2. Валидация пути, извлечение id.
		attachmentID := chi.URLParam(r, "id")
		if attachmentID == "" {
			thttp.WriteError(w, models.NewBadRequest(map[string]any{"id": "missing attachment"}))
			return
		}
		start := time.Now()

		// 3. Доменная логика.
		meta, rc, err := svc.Download(r.Context(), ownerID, attachmentID)
		if err != nil {
			thttp.WriteError(w, err)
			return
		}
		defer rc.Close()
		// 4. Подготовка заголовков ответа.
		// Content-Type — только если известен.
		if ct := meta.ContentType; ct != "" {
			w.Header().Set("Content-Type", ct)
		}

		if meta.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
		}
		if meta.FileName != "" {
			w.Header().Set("Content-Disposition", `attachment; filename="`+meta.FileName+`"`)
		} else {
			w.Header().Set("Content-Disposition", "attachment")
		}
		w.WriteHeader(http.StatusOK)
		// 5. тримим тело. Ошибку записи клиенту обычно логируем.
		if _, err := io.Copy(w, rc); err != nil {
			// не шлём ещё один ответ — просто залогируем
			log.Infow("attachment download client disconnected",
				"user", ownerID, "att", attachmentID, "err", err,
			)
			return
		}

		// 6) Логируем успех с таймингом.
		log.Infow("attachment download ok",
			"user", ownerID,
			"att", attachmentID,
			"size", meta.Size,
			"mime", meta.ContentType,
			"took", time.Since(start),
		)
	}
}

// NewListAttachments — обработчик GET /api/attachments?secret_id=&limit=&offset=
func NewListAttachments(svc AttachmentService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Достать ownerID из контекста.
		ownerID, _ := r.Context().Value(middlewares.UserLoginKey).(string)
		if ownerID == "" {
			thttp.WriteError(w, models.NewUnauthorized(nil))
			return
		}
		// 2. Чтение query.
		q := r.URL.Query()
		secretID := q.Get("secret_id")
		limit := q.Get("limit")
		offset := q.Get("offset")
		// 3. Валидация query.
		if secretID == "" {
			thttp.WriteError(w, models.NewBadRequest(map[string]any{"id": "missing attachment"}))
			return
		}
		limitInt, err := strconv.Atoi(limit)
		if limit == "" || err != nil || limitInt <= 0 || limitInt > 100 {
			limitInt = 20
		}
		offsetInt, err := strconv.Atoi(offset)
		if offset == "" || err != nil || offsetInt < 0 {
			offsetInt = 0
		}

		// 4. Вызываем сервис.
		items, total, err := svc.List(r.Context(), ownerID, secretID, limitInt, offsetInt)
		if err != nil {
			thttp.WriteError(w, err)
			return
		}
		// 5. Формируем DTO-ответ.
		resp := toAttachmentList(items, total, limitInt, offsetInt)
		// 6. Заголовки и ответ.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Infow("encode list attachments failed", "err", err)
			return
		}
	}
}

func NewPing(svc AttachmentService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Ping(r.Context()); err != nil {
			log.Errorf("Failed to open DataBase: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
