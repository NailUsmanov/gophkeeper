// Package handlers описывает функции обработчики, используемые в HTTP-запросах.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/middlewares"
	"github.com/NailUsmanov/gophkeeper/internal/server/service/secret"
	thttp "github.com/NailUsmanov/gophkeeper/internal/server/transport/http"
	"go.uber.org/zap"
)

// SecretHandler держит зависимости уровня HTTP: сервис и логгер.
type SecretHandler struct {
	svc secret.SecretService // Handler будет использовать интерфейс SecretService, а тот ServiceRepository
	log *zap.SugaredLogger
}

// NewSecretHandler — конструктор хендлера.
func NewSecretHandler(svc secret.SecretService, log *zap.SugaredLogger) *SecretHandler {
	return &SecretHandler{
		svc: svc,
		log: log,
	}
}

// Create обрабатывает POST /api/v1/secrets
func (sh *SecretHandler) Create(w http.ResponseWriter, r *http.Request) {

	// 1) DTO-вход
	var req thttp.CreateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sh.log.Error("decode request JSON body:", err)
		thttp.WriteError(w, models.NewInvalidInput(map[string]any{"body": "bad json"}))
		return
	}
	if len(req.Type) == 0 || len(req.Title) == 0 || len(req.Data) == 0 {
		thttp.WriteError(w, models.NewInvalidInput(map[string]any{"fields": "type/title/data required"}))
		return
	}

	// 2) userID из контекста (middleware его положил)
	userID, _ := r.Context().Value(middlewares.UserLoginKey).(string)
	if userID == "" {
		thttp.WriteError(w, models.NewUnauthorized(nil))
		return
	}

	// 3) DTO → домен
	domainReq := secret.CreateReq{
		Type:  models.SecretType(req.Type),
		Title: req.Title,
		Data:  req.Data,
	}

	// 4) доменная логика
	newSecret, err := sh.svc.Create(r.Context(), userID, domainReq)
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
		sh.log.Error("error encoding response")
	}

}
