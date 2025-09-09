// Package handlers описывает HTTP-хендлеры.
//
// handler_auth — регистрация пользователя: чтение входа, вызов домена,
// установка cookie с токеном и возврат безопасного ответа.

//go:generate mockgen -source=handler_auth.go -destination=handler_auth_mocks_test.go -package=handlers_test
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	thttp "github.com/NailUsmanov/gophkeeper/internal/server/transport/http"
	"go.uber.org/zap"
)

// AuthService описывает доменный сервис аутентификации.
// Хендлеры обращаются к нему, не зная деталей БД/токенов/хэширования.
type AuthService interface {
	Register(ctx context.Context, email, password string) (*models.User, string, error)
	Login(ctx context.Context, email, password string) (*models.User, string, error)
	Logout(ctx context.Context, authToken string) error
}

// NewAuth обрабатывает POST /api/v1/register.
func NewRegister(svc AuthService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) Считываем email и пароль из JSON-тела.
		var req thttp.AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
			req.Email == "" || req.Password == "" {
			log.Warnw("invalid register payload", "err", err)
			thttp.WriteError(w, models.NewInvalidInput(map[string]any{"email/password": "required"}))
			return
		}

		// 2) Создаем пользователя и выдаем токен (доменная логика).
		user, token, err := svc.Register(r.Context(), req.Email, req.Password)
		if err != nil {
			log.Error("Register failed")
			thttp.WriteError(w, models.NewInternal(nil))
			return
		}

		// 3) Ставим cookie c токеном(HttpOnly, SameSite, Secure)
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false,
			Expires:  time.Now().Add(24 * time.Hour),
		})

		// 4) Домен -> DTO-ответ.
		resp := thttp.UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		}

		// 4) Ответ.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err = json.NewEncoder(w).Encode(resp); err != nil {
			log.Error("register response encode failed")
			return
		}

	}
}

// NewLogin обрабатывает POST /api/v1/login.
func NewLogin(svc AuthService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) Считываем email и пароль из JSON-тела.
		var req thttp.AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
			req.Email == "" || req.Password == "" {
			log.Warnw("invalid login payload", "err", err)
			thttp.WriteError(w, err)
			return
		}

		// 2) Проводим авторизацию пользователя и выдачу токена.
		user, token, err := svc.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			log.Error("Login failed")
			thttp.WriteError(w, models.NewInternal(nil))
			return
		}

		// 3) Ставим cookie c токеном(HttpOnly, SameSite, Secure)
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false,
			Expires:  time.Now().Add(24 * time.Hour),
		})

		// 4) Домен -> DTO-ответ.
		resp := thttp.UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		}

		// 5) Ответ.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err = json.NewEncoder(w).Encode(resp); err != nil {
			log.Error("login response encode failed")
			return
		}

	}
}

// NewLogout обрабатывает POST /api/v1/logout.
// Идемпотентен: при отсутствии/некорректности токена всё равно стирает cookie и возвращает 204.
func NewLogout(svc AuthService, log *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Читаем токен из cookie.
		c, _ := r.Cookie("auth_token")
		token := ""
		if c != nil {
			token = c.Value
		}

		// 2. Пытаемся отозвать.
		if token != "" {
			if err := svc.Logout(r.Context(), token); err != nil {
				log.Infow("logout revoke error (ignored for idempotence)", "err", err)
			}
		}

		// 3. Стираем cookie на клиенте.
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false,
			MaxAge:   -1,
		})

		// 4. Ответ.
		w.WriteHeader(http.StatusNoContent)

	}
}
