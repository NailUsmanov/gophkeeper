// Package http содержит ErrorResponse и хелпер WriteError для единообразных ответов об ошибках.
package http

import (
	"encoding/json"
	"net/http"

	"github.com/NailUsmanov/gophkeeper/internal/models"
)

// ErrorResponse описывает ответ об ошибке.
type ErrorResponse struct {
	Error   string         `json:"error"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// WriteError маппит любую ошибку error в JSON-ответ по контракту.
// Если это не AppError - заворачиваем в 500 internal.
func WriteError(w http.ResponseWriter, err error) {
	app, ok := err.(*models.AppError)
	if !ok {
		app = models.NewInternal(map[string]any{"cause": err.Error()})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(app.HTTPStatus)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   app.Code,
		Message: app.Message,
		Details: app.Details,
	})
}
