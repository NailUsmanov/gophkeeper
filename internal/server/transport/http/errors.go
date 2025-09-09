// Package http содержит ErrorResponse и хелпер WriteError для единообразных ответов об ошибках.
package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/NailUsmanov/gophkeeper/internal/models"
)

// ErrorResponse описывает ответ об ошибке.
type ErrorResponse struct {
	Error   string         `json:"error"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// WriteError маппит любую ошибку в JSON-ответ по контракту.
// Правила:
//  1. Если err уже *models.AppError (в т.ч. завернутая) — отдаем как есть.
//  2. Если это известная sentinel-ошибка — маппим на соответствующий AppError.
//  3. Иначе — Internal (500).
func WriteError(w http.ResponseWriter, err error) {
	// 1) Попробуем распаковать *models.AppError (errors.As — важен)
	var app *models.AppError
	if errors.As(err, &app) {
		writeAppError(w, app)
		return
	}

	// 2) Маппинг sentinels -> AppError
	switch {
	case errors.Is(err, models.ErrCodeUnauthorized):
		writeAppError(w, models.NewUnauthorized(nil))
		return
	case errors.Is(err, models.ErrCodeNotFound):
		writeAppError(w, models.NewNotFound(nil))
		return
	case errors.Is(err, models.ErrCodeAlreadyExists):
		writeAppError(w, models.NewAlreadyExists(nil))
		return
	case errors.Is(err, models.ErrCodeInvalidInput):
		writeAppError(w, models.NewInvalidInput(nil))
		return
	case errors.Is(err, models.ErrCodeConflict):
		writeAppError(w, models.NewConflict(nil))
	case errors.Is(err, models.ErrCodeForbidden):
		writeAppError(w, models.NewForbidden(nil))
	}

	// 3) Фоллбэк — Internal, не светим лишних деталей наружу
	writeAppError(w, models.NewInternal(nil))
}

func writeAppError(w http.ResponseWriter, app *models.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(app.HTTPStatus)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   app.Code,
		Message: app.Message,
		Details: app.Details,
	})
}
