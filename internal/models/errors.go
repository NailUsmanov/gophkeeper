// Package models содержит сущность AppError для описания ошибок домена.

package models

import "net/http"

const (
	ErrCodeInvalidInput   = "invalid_input"
	ErrCodeUnauthorized   = "unauthorized"
	ErrCodeForbidden      = "forbidden"
	ErrCodeNotFound       = "not_found"
	ErrCodeConflict       = "conflict"
	ErrCodeValidationFail = "validation_failed"
	ErrCodeInternal       = "internal_error"
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
	return e.Message
}

// NewInvalidInput — 400 Bad Request (ошибка формата/структуры запроса).
func NewInvalidInput(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeInvalidInput,
		Message:    "invalid input",
		Details:    details,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewUnauthorized — 401 Unauthorized (нет/невалидный токен).
func NewUnauthorized(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeUnauthorized,
		Message:    "not authorized",
		Details:    details,
		HTTPStatus: http.StatusUnauthorized,
	}
}

// NewForbidden — 403 Forbidden (доступ к ресурсу запрещён при валидной аутентификации).
func NewForbidden(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeForbidden,
		Message:    "forbidden",
		Details:    details,
		HTTPStatus: http.StatusForbidden,
	}
}

// NewNotFound — 404 Not Found.
func NewNotFound(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeNotFound,
		Message:    "not found",
		Details:    details,
		HTTPStatus: http.StatusNotFound,
	}
}

// NewConflict — 409 Conflict конфликт версий при optimistic locking.
func NewConflict(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeConflict,
		Message:    "conflict",
		Details:    details,
		HTTPStatus: http.StatusConflict,
	}
}

// NewValidationFailed — 422 бизнес-валидация не пройдена.
func NewValidationFailed(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeValidationFail,
		Message:    "validation failed",
		Details:    details,
		HTTPStatus: http.StatusUnprocessableEntity,
	}
}

// NewInternal — 500 Internal Server Error (непредвиденная внутренняя ошибка).
func NewInternal(details map[string]any) *AppError {
	return &AppError{
		Code:       ErrCodeInternal,
		Message:    "internal server error",
		Details:    details,
		HTTPStatus: http.StatusInternalServerError,
	}
}
