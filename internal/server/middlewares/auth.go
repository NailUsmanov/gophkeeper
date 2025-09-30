// Package middlewares содержит middleware-функции для HTTP-сервера.
// Здесь реализуются:
//   - логирование запросов;
//   - сжатие ответов (gzip);
//   - аутентификация пользователей через cookie.
//
// AuthMiddleware проверяет наличие и валидность токена в cookie "auth_token".
// Если токен корректный — в контекст запроса добавляется userID,
// который потом смогут прочитать хендлеры.
package middlewares

import (
	"context"
	"net/http"
)

// contextLogin — тип-обёртка для ключа контекста.
// Используем отдельный тип, чтобы избежать коллизий с другими ключами.
type contextLogin string

// UserLoginKey — ключ, по которому в контексте хранится идентификатор пользователя.
// Хендлеры могут получить userID так:
//
//	userID, _ := r.Context().Value(middlewares.UserLoginKey).(string)
const UserLoginKey contextLogin = "userID"

// TokenValidator определяет интерфейс для проверки токена.
// Validate должен вернуть userID, если токен корректный, или ошибку — если нет.
type TokenValidator interface {
	Validate(ctx context.Context, token string) (string, error) //-> userID
}

// AuthMiddleware - HTTP middleware, проводит аутентификацию пользователя.
// Она выполняет:
//  1. Достаёт cookie "auth_token".
//  2. Валидирует токен через TokenValidator.
//  3. Если токен валиден — добавляет userID в контекст и передаёт запрос дальше.
//  4. Если токен невалиден — отвечает 401 Unauthorized.
func AuthMiddleWare(tokens TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1) Достаём cookie
			cookie, err := r.Cookie("auth_token")
			if err != nil || cookie.Value == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			// 2) Валидируем токен → получаем userID
			userID, err := tokens.Validate(r.Context(), cookie.Value)
			if err != nil || userID == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			// 3) Кладём userID в контекст
			ctx := context.WithValue(r.Context(), UserLoginKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
