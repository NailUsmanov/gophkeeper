// Package models содержит сущность AppError для описания ошибок домена.

package models

import (
	"errors"
	"net/http"
)

var (
	ErrCodeInvalidInput   = errors.New("invalid_input")
	ErrCodeUnauthorized   = errors.New("unauthorized")
	ErrCodeForbidden      = errors.New("forbidden")
	ErrCodeNotFound       = errors.New("not_found")
	ErrCodeConflict       = errors.New("conflict")
	ErrCodeValidationFail = errors.New("validation_failed")
	ErrCodeInternal       = errors.New("internal_error")
	ErrCodeAlreadyExists  = errors.New("already exists")
	ErrCodeBadRequest     = errors.New("bad_request")
)

// AppError — доменная ошибка: короткий код, человекочитаемое сообщение,
// произвольные детали (опционально) и соответствующий HTTP-статус.
type AppError struct {
	Code       string         // короткий код: "invalid_input", "unauthorized", "not_found", ...
	Message    string         // описание ошибки
	Details    map[string]any // дополнительные сведения (например, {"field":"too long"})
	HTTPStatus int            // HTTP-статус: 400/401/404/422/...
}

// Error реализует интерфейс error, возвращая описание-сообщение.
func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

// Status возвращает статус-код ошибки.
func (e *AppError) Status() int {
	return e.HTTPStatus
}

// NewInvalidInput — 400 Bad Request (ошибка формата/структуры запроса).
func NewInvalidInput(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeInvalidInput.Error(),
		Message:    "invalid input",
		Details:    details,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewUnauthorized — 401 Unauthorized (нет/невалидный токен).
func NewUnauthorized(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeUnauthorized.Error(),
		Message:    "not authorized",
		Details:    details,
		HTTPStatus: http.StatusUnauthorized,
	}
}

// NewForbidden — 403 Forbidden (доступ к ресурсу запрещён при валидной аутентификации).
func NewForbidden(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeForbidden.Error(),
		Message:    "forbidden",
		Details:    details,
		HTTPStatus: http.StatusForbidden,
	}
}

// NewNotFound — 404 Not Found.
func NewNotFound(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeNotFound.Error(),
		Message:    "not found",
		Details:    details,
		HTTPStatus: http.StatusNotFound,
	}
}

// NewConflict — 409 Conflict конфликт версий при optimistic locking.
func NewConflict(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeConflict.Error(),
		Message:    "conflict",
		Details:    details,
		HTTPStatus: http.StatusConflict,
	}
}

// NewValidationFailed — 422 бизнес-валидация не пройдена.
func NewValidation(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeValidationFail.Error(),
		Message:    "validation failed",
		Details:    details,
		HTTPStatus: http.StatusUnprocessableEntity,
	}
}

// NewInternal — 500 Internal Server Error (непредвиденная внутренняя ошибка).
func NewInternal(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeInternal.Error(),
		Message:    "internal server error",
		Details:    details,
		HTTPStatus: http.StatusInternalServerError,
	}
}

func NewAlreadyExists(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeAlreadyExists.Error(),
		Message:    "already exists",
		Details:    details,
		HTTPStatus: http.StatusConflict,
	}
}

func NewBadRequest(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeBadRequest.Error(),
		Message:    "bad request",
		Details:    details,
		HTTPStatus: http.StatusBadRequest,
	}
}

// HasCode извлекает из err код приложения (если он есть) и сравнивает.
func HasCode(err error, want string) bool {
	var app *AppError
	if errors.As(err, &app) {
		return app.Code == want
	}
	return false
}
