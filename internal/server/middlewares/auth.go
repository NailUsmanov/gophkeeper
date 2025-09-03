// Package middlewares содержит middleware-функции для HTTP-сервера.

// Включает логирование запросов, сжатие ответов и аутентификацию пользователей.
package middlewares

import (
	"context"
	"net/http"
	"strconv"
)

// Ключ для хранения userID в контексте.
type contextLogin string

// UserIDKey используется для передачи userID в контексте.
const (
	UserLoginKey contextLogin = "userID"
)

// AuthMiddleware - HTTP middleware, проводит аутентификацию пользователя.
// Если кука есть, то используется ее значение как UserID
// Затем она добавляется в контекст.
func AuthMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Проверяем куку auth_token.
		coockie, err := r.Cookie("auth_token")
		if err != nil || coockie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// 2. Если кука есть - используем ее значение как login.
		userID, err := strconv.Atoi(coockie.Value)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// 3. Добавляем login в контекст
		ctx := context.WithValue(r.Context(), UserLoginKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
