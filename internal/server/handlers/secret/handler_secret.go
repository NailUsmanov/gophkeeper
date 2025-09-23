// Package handlers описывает функции обработчики, используемые в HTTP-запросах.
//
// Здесь описывается реализация обработчиков для создания секрета, получения, выдачи списком
// и обновления данных секрета.
//
//go:generate mockgen -source=handler_secret.go -destination=handler_secret_mocks_test.go -package=handlers_test
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/middlewares"
	"github.com/NailUsmanov/gophkeeper/internal/server/service/secret"
	thttp "github.com/NailUsmanov/gophkeeper/internal/server/transport/http"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// CreateSecretService интерфейс для создания секрета.
type CreateSecretService interface {
	Create(ctx context.Context, ownerID string, req secret.CreateReq) (*secret.Secret, error)
}

// Create обрабатывает POST /api/v1/secrets.
func NewCreateSecret(svc CreateSecretService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) userID из контекста (middleware его положил)
		userID, _ := r.Context().Value(middlewares.UserLoginKey).(string)
		if userID == "" {
			thttp.WriteError(w, models.NewUnauthorized(nil))
			return
		}
		// 2) DTO-вход
		var req thttp.CreateSecretRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error("decode request JSON body:", err)
			thttp.WriteError(w, models.NewInvalidInput(map[string]any{"body": "bad json"}))
			return
		}
		if len(req.Type) == 0 || len(req.Title) == 0 || len(req.Data) == 0 {
			thttp.WriteError(w, models.NewInvalidInput(map[string]any{"fields": "type/title/data required"}))
			return
		}

		// 3) DTO → домен
		domainReq := secret.CreateReq{
			Type:  models.SecretType(req.Type),
			Title: req.Title,
			Data:  req.Data,
		}

		// 4) доменная логика
		newSecret, err := svc.Create(r.Context(), userID, domainReq)
		if err != nil {
			thttp.WriteError(w, err)
			return
		}

		// 5) домен → DTO-ответ
		resp := thttp.SecretResponse{
			ID:        newSecret.ID,
			Type:      string(newSecret.Type),
			Title:     newSecret.Title,
			Data:      newSecret.Data,
			Version:   newSecret.Version,
			CreatedAt: newSecret.CreatedAt.Format(time.RFC3339),
			UpdatedAt: newSecret.UpdatedAt.Format(time.RFC3339),
		}

		// 6) ответ
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error("error encoding response")
			return
		}
	}
}

// GetSecretService интерфейс для получения секрета по ID.
type GetSecretService interface {
	GetByID(ctx context.Context, ownerID, secretID string) (*secret.Secret, error)
	List(ctx context.Context, ownerID string, limit, offset int, filter secret.SecretListFilter) ([]*secret.Secret, int, error) // по умолчанию возвращает только не удалённые (DeletedAt == nil)
}

// GetByID обрабатывает GET /api/v1/secrets/{id}.
func NewGetByID(svc GetSecretService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Достаем id из chi
		id := chi.URLParam(r, "id")
		if id == "" {
			thttp.WriteError(w, models.NewInvalidInput(map[string]any{"id": "empty"}))
			return
		}

		// 2) userID из контекста (middleware его положил).
		userID, _ := r.Context().Value(middlewares.UserLoginKey).(string)
		if userID == "" {
			thttp.WriteError(w, models.NewUnauthorized(nil))
			return
		}

		// 3) доменная логика
		secret, err := svc.GetByID(r.Context(), userID, id)
		if err != nil {
			thttp.WriteError(w, err)
			return
		}

		// 4) Домен -> DTO-ответ
		resp := thttp.SecretResponse{
			ID:        secret.ID,
			Type:      string(secret.Type),
			Title:     secret.Title,
			Data:      secret.Data,
			Version:   secret.Version,
			CreatedAt: secret.CreatedAt.Format(time.RFC3339),
			UpdatedAt: secret.UpdatedAt.Format(time.RFC3339),
		}

		// 5) Ответ
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(&resp); err != nil {
			log.Error("error encoding response")
			return
		}
	}
}

// List обрабатывает GET /api/v1/secrets?limit=&offset=&type=&updated_after=
func NewList(svc GetSecretService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Берем userID из контекста.
		userID, _ := r.Context().Value(middlewares.UserLoginKey).(string)
		if userID == "" {
			log.Error("empty user")
			thttp.WriteError(w, models.NewUnauthorized(nil))
			return
		}

		// 2. Считываем query параметры и валидируем их
		q := r.URL.Query()
		limit := q.Get("limit")
		offset := q.Get("offset")
		typ := q.Get("type")
		update := q.Get("updated_after")

		// если параметры отсутствуют, то выставляем начальные по умолчанию
		if limit == "" {
			limit = "20"
		}
		if offset == "" {
			offset = "0"
		}

		limitInt, err := strconv.Atoi(limit)
		if err != nil || limitInt < 1 || limitInt > 100 {
			thttp.WriteError(w, models.NewInvalidInput(map[string]any{"limit": "must be 1..100"}))
			return
		}
		offsetInt, err := strconv.Atoi(offset)
		if err != nil || offsetInt < 0 {
			thttp.WriteError(w, models.NewInvalidInput(map[string]any{"offset": "must be >= 0"}))
			return
		}

		// 3. Собираем SecretListFilter
		var filter secret.SecretListFilter
		// Проверяем тип
		if typ != "" {
			if typ != "password" && typ != "note" && typ != "card" && typ != "file" {
				log.Error("not correct type")
				thttp.WriteError(w, models.NewInvalidInput(map[string]any{"type": "incorrect"}))
				return
			}
			ty := models.SecretType(typ)
			filter.Type = &ty
		}

		// Проверяем время
		if update != "" {
			t, err := time.Parse(time.RFC3339, update)
			if err != nil {
				log.Error("failed parsed updated_at")
				thttp.WriteError(w, models.NewInvalidInput(map[string]any{"updated_after": "must be RFC3339"}))
				return
			}
			filter.UpdatedAfter = &t
		}

		// 4. Доменная логика
		items, total, err := svc.List(r.Context(), userID, limitInt, offsetInt, filter)
		if err != nil {
			thttp.WriteError(w, err)
			return
		}

		// 5. Складываем данные в DTO
		var result thttp.ListSecretResponse
		for _, v := range items {
			sec := thttp.SecretResponse{
				ID:        v.ID,
				Type:      string(v.Type),
				Title:     v.Title,
				Data:      v.Data,
				Version:   v.Version,
				CreatedAt: v.CreatedAt.Format(time.RFC3339),
				UpdatedAt: v.UpdatedAt.Format(time.RFC3339),
			}
			result.Items = append(result.Items, sec)
		}
		// 6. Собираем ответ и отправляем.
		result.Limit = limitInt
		result.Offset = offsetInt
		result.Total = total
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Error("encoding failed")
		}
	}
}

type UpdateService interface {
	Update(ctx context.Context, ownerID, secretID string, req secret.UpdateReq) (*secret.Secret, error)
}

// Update обрабатывает PUT /api/v1/secrets/{id}
func NewUpdate(svc UpdateService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Берем userID из контекста.
		log.Info("Достаем user")
		userID, _ := r.Context().Value(middlewares.UserLoginKey).(string)
		if userID == "" {
			log.Error("empty user")
			thttp.WriteError(w, models.NewUnauthorized(nil))
			return
		}
		// 2. Достаем {id} из path.
		log.Info("Достаем id секрета")
		secretID := chi.URLParam(r, "id")

		if secretID == "" {
			log.Error("empty secret ID")
			thttp.WriteError(w, models.NewValidation(map[string]any{"id": "required"}))
			return
		}

		// 3. Декодим JSON в UpdateSecretRequest.
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer r.Body.Close()

		var sec thttp.UpdateSecretRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&sec); err != nil {
			log.Error("trouble with encoding")
			thttp.WriteError(w, models.NewValidation(nil))
			return
		}
		if dec.More() {
			thttp.WriteError(w, models.NewValidation(map[string]any{"body": "multiple json objects"}))
			return
		}

		// 4. Валидация DTO.
		if sec.Version <= 0 {
			log.Error("version conflict")
			thttp.WriteError(w, models.NewValidation(map[string]any{"version": "must be > 0"}))
			return
		}
		if len(sec.Data) == 0 {
			log.Error("empty input")
			thttp.WriteError(w, models.NewValidation(map[string]any{"data": "required"}))
			return
		}

		if sec.Title != nil {
			t := strings.TrimSpace(*sec.Title)
			if t == "" {
				thttp.WriteError(w, models.NewValidation(map[string]any{"title": "cannot be empty if provided"}))
				return
			}
			sec.Title = &t
		}
		// 5. Маппинг DTO -> домен UpdateReq.
		req := &secret.UpdateReq{
			Title:   sec.Title,
			Data:    sec.Data,
			Version: sec.Version,
		}

		// 6. Вызываем сервисный слой.
		newSecret, err := svc.Update(r.Context(), userID, secretID, *req)
		if err != nil {
			switch {
			case models.HasCode(err, models.ErrCodeConflict.Error()):
				thttp.WriteError(w, models.NewConflict(nil))
			case models.HasCode(err, models.ErrCodeNotFound.Error()):
				thttp.WriteError(w, models.NewNotFound(nil))
			case models.HasCode(err, models.ErrCodeUnauthorized.Error()):
				thttp.WriteError(w, models.NewUnauthorized(nil))
			case models.HasCode(err, models.ErrCodeValidationFail.Error()), models.HasCode(err, models.ErrCodeInvalidInput.Error()):
				thttp.WriteError(w, models.NewValidation(nil))
			default:
				thttp.WriteError(w, models.NewInternal(nil))
			}
			log.Infow("update failed",
				"userID", userID,
				"secretID", secretID,
				"version", sec.Version,
				"err", err,
			)
			return
		}

		// 7. Маппим домен UpdateReq -> SecretResponse.
		resp := thttp.SecretResponse{
			ID:        newSecret.ID,
			Type:      string(newSecret.Type),
			Title:     newSecret.Title,
			Data:      newSecret.Data,
			Version:   newSecret.Version,
			CreatedAt: newSecret.CreatedAt.Format(time.RFC3339),
			UpdatedAt: newSecret.UpdatedAt.Format(time.RFC3339),
		}

		// 8. Отправляю ответ.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
