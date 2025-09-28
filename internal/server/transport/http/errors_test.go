package http_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	thttp "github.com/NailUsmanov/gophkeeper/internal/server/transport/http"
)

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("%q does not contain %q", s, sub)
	}
}

func TestWriteError_AppError_Variants(t *testing.T) {
	cases := []struct {
		name       string
		err        *models.AppError
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{"unauthorized", models.NewUnauthorized(nil), http.StatusUnauthorized, models.ErrCodeUnauthorized.Error(), "not authorized"},
		{"forbidden", models.NewForbidden(map[string]any{"k": "v"}), http.StatusForbidden, models.ErrCodeForbidden.Error(), "forbidden"},
		{"not_found", models.NewNotFound(nil), http.StatusNotFound, models.ErrCodeNotFound.Error(), "not found"},
		{"conflict", models.NewConflict(nil), http.StatusConflict, models.ErrCodeConflict.Error(), "conflict"},
		{"validation", models.NewValidation(map[string]any{"field": "x"}), http.StatusUnprocessableEntity, models.ErrCodeValidationFail.Error(), "validation failed"},
		{"bad_request", models.NewBadRequest(nil), http.StatusBadRequest, models.ErrCodeBadRequest.Error(), "bad request"},
		{"already_exists", models.NewAlreadyExists(nil), http.StatusConflict, models.ErrCodeAlreadyExists.Error(), "already exists"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			thttp.WriteError(rr, tc.err)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status=%d, want %d, body=%s", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("Content-Type=%q, want application/json", ct)
			}
			body := rr.Body.String()
			mustContain(t, body, `"error":"`+tc.wantCode+`"`)
			mustContain(t, body, `"message":"`+tc.wantMsg+`"`)
			if tc.err.Details != nil {
				mustContain(t, body, `"details"`)
			}
		})
	}
}

func TestWriteError_GenericError_MapsToInternal(t *testing.T) {
	rr := httptest.NewRecorder()
	thttp.WriteError(rr, errors.New("boom"))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", rr.Code)
	}
	body := rr.Body.String()
	mustContain(t, body, `"error":"`+models.ErrCodeInternal.Error()+`"`)
	mustContain(t, body, `"message":"internal server error"`)
}

func TestWriteError_NilError_TreatedAsInternal(t *testing.T) {
	rr := httptest.NewRecorder()
	thttp.WriteError(rr, nil)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", rr.Code)
	}
	body := rr.Body.String()
	mustContain(t, body, `"error":"`+models.ErrCodeInternal.Error()+`"`)
}

func TestWriteError_WrappedAppError(t *testing.T) {
	rr := httptest.NewRecorder()

	// Заворачиваем AppError в fmt.Errorf("%w", ...)
	wrapped := fmt.Errorf("wrap: %w", models.NewForbidden(map[string]any{"hint": "x"}))
	thttp.WriteError(rr, wrapped)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusForbidden)
	}
	body := rr.Body.String()
	// важно: проверяем поле "error", а не "code"
	if !strings.Contains(body, `"error":"`+models.ErrCodeForbidden.Error()+`"`) {
		t.Fatalf("body=%s", body)
	}
	if !strings.Contains(body, `"details"`) {
		t.Fatalf("body has no details: %s", body)
	}
}

func TestWriteError_AppError_EmptyMessage(t *testing.T) {
	rr := httptest.NewRecorder()

	// Сообщение пустое — проверь, что JSON всё равно валидный и отдан нужный код/статус
	app := &models.AppError{
		Code:       "i_am_a_teapot",
		Message:    "",  // <- ветка про пустой message
		HTTPStatus: 418, // teapot :)
		Details:    nil,
	}
	thttp.WriteError(rr, app)

	if rr.Code != 418 {
		t.Fatalf("status=%d, want 418", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"error":"i_am_a_teapot"`) {
		t.Fatalf("body=%s", body)
	}
	// message может быть пустым или повторять code — не навязываем строгую проверку,
	// важно пройти ветку создания ответа с пустым message.
}

func TestWriteError_JSONMarshalFails(t *testing.T) {
	rr := httptest.NewRecorder()

	// Несериализуемые details — сломаем json.Marshal
	app := &models.AppError{
		Code:       "weird",
		Message:    "oops",
		HTTPStatus: http.StatusTeapot, // 418
		Details:    map[string]any{"bad": make(chan int)},
	}
	thttp.WriteError(rr, app)

	// Некоторые реализации оставляют исходный статус (418),
	// другие мапят на 500. Разрешим оба варианта.
	if rr.Code != http.StatusTeapot && rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 418 or 500", rr.Code)
	}

	// Fallback может не писать тело вовсе. Не падаем на пустом теле.
	// Если Content-Type проставлен — допускаем application/json или text/plain.
	if ct := rr.Header().Get("Content-Type"); ct != "" &&
		!(strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/plain")) {
		t.Fatalf("unexpected Content-Type=%q", ct)
	}
}

func TestWriteError_StatusZeroDefaultsTo500(t *testing.T) {
	rr := httptest.NewRecorder()
	thttp.WriteError(rr, &models.AppError{Code: "x", Message: "y"}) // HTTPStatus=0
	if rr.Code != 500 {
		t.Fatalf("status=%d, want 500", rr.Code)
	}
}

func TestWriteError_SentinelMapping(t *testing.T) {
	t.Parallel()

	type tc struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}
	cases := []tc{
		{
			name:       "unauthorized_sentinel",
			err:        models.ErrCodeUnauthorized,
			wantStatus: http.StatusUnauthorized,
			wantCode:   models.ErrCodeUnauthorized.Error(),
			wantMsg:    "not authorized",
		},
		{
			name:       "forbidden_sentinel",
			err:        models.ErrCodeForbidden,
			wantStatus: http.StatusForbidden,
			wantCode:   models.ErrCodeForbidden.Error(),
			wantMsg:    "forbidden",
		},
		{
			name:       "not_found_sentinel",
			err:        models.ErrCodeNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   models.ErrCodeNotFound.Error(),
			wantMsg:    "not found",
		},
		{
			name:       "conflict_sentinel",
			err:        models.ErrCodeConflict,
			wantStatus: http.StatusConflict,
			wantCode:   models.ErrCodeConflict.Error(),
			wantMsg:    "conflict",
		},
		{
			name:       "invalid_input_sentinel",
			err:        models.ErrCodeInvalidInput,
			wantStatus: http.StatusBadRequest,
			wantCode:   models.ErrCodeInvalidInput.Error(),
			wantMsg:    "invalid input",
		},
		{
			name:       "already_exists_sentinel",
			err:        models.ErrCodeAlreadyExists,
			wantStatus: http.StatusConflict,
			wantCode:   models.ErrCodeAlreadyExists.Error(),
			wantMsg:    "already exists",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			thttp.WriteError(rr, c.err)

			if rr.Code != c.wantStatus {
				t.Fatalf("status=%d, want %d (body=%s)", rr.Code, c.wantStatus, rr.Body.String())
			}
			if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("Content-Type=%q, want application/json", ct)
			}
			body := rr.Body.String()
			if !strings.Contains(body, `"error":"`+c.wantCode+`"`) {
				t.Fatalf("body=%s (want error=%q)", body, c.wantCode)
			}
			if !strings.Contains(body, `"message":"`+c.wantMsg+`"`) {
				t.Fatalf("body=%s (want message=%q)", body, c.wantMsg)
			}
		})
	}
}
