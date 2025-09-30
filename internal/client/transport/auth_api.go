// Package transport используется для выполнения запросов к серверу.
//
// auth_api.go используется для регистрации, логирования и выхода пользователя из сессии.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// AuthRequest — DTO, которое сервер ждёт в /api/v1/login и /api/v1/register.
type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UserResponse - DTO ответа сервера при login/register.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Login - отправляет запрос на функцию логирования пользователя к серверу.
func (c *Client) Login(ctx context.Context, email, password string) (*UserResponse, string, error) {
	// 1. Собираем URL.
	ep := c.BaseURL.ResolveReference(&url.URL{Path: "/api/v1/login"})

	// 2. JSON-тело запроса делаем.
	body, err := json.Marshal(AuthRequest{Email: email, Password: password})
	if err != nil {
		return nil, "", fmt.Errorf("marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.String(), bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 3) выполняем запрос и получаем ответ.
	res, err := c.do(req)
	if err != nil {
		return nil, "", fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()

	// 4) обрабатываем коды
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("login failed: %d", res.StatusCode)
	}

	// 5) Парсим JSON ответа.
	var user UserResponse
	if err := json.NewDecoder(res.Body).Decode(&user); err != nil {
		return nil, "", fmt.Errorf("decode user: %w", err)
	}

	// 6) достаём токен из Set-Cookie
	var token string
	for _, sc := range res.Cookies() {
		if sc.Name == "auth_token" && sc.Value != "" {
			token = sc.Value
			break
		}
	}
	if token == "" {
		return &user, "", fmt.Errorf("no auth_token cookie in response")
	}
	return &user, token, nil
}

// Register - реализует запрос на регистрацию пользователя.
// Возвращает пользователя, токен, ошибку.
func (c *Client) Register(ctx context.Context, email, password string) (*UserResponse, string, error) {
	// 1. Собираем URL.
	ep := c.BaseURL.ResolveReference(&url.URL{Path: "/api/v1/register"})

	// 2. JSON-тело запроса делаем и составляем запрос.
	body, err := json.Marshal(AuthRequest{Email: email, Password: password})
	if err != nil {
		return nil, "", fmt.Errorf("marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.String(), bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 3) выполняем запрос и получаем ответ.
	resp, err := c.do(req)
	if err != nil {
		return nil, "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// 4) обрабатываем коды
	switch resp.StatusCode {
	case http.StatusCreated:
		// всё ок — идём дальше
	case http.StatusBadRequest:
		return nil, "", fmt.Errorf("invalid input (400 Bad Request)")
	case http.StatusConflict:
		return nil, "", fmt.Errorf("email already exists (409 Conflict)")
	case http.StatusInternalServerError:
		return nil, "", fmt.Errorf("server error (500 Internal Server Error)")
	default:
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// 5) Обрабатываем JSON ответ.
	var user UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, "", fmt.Errorf("decode user: %w", err)
	}

	// 6) Достаем токен из Set Cookie.
	var token string
	for _, sc := range resp.Cookies() {
		if sc.Name == "auth_token" && sc.Value != "" {
			token = sc.Value
			break
		}
	}
	if token == "" {
		return &user, "", fmt.Errorf("no auth_token cookie in response")
	}
	return &user, token, nil
}

// Logout отправляет запрос на разрыв сессии с конкретным пользоватем по токену.
func (c *Client) Logout(ctx context.Context, token string) error {
	// 1. Собираем URL.
	ep := c.BaseURL.ResolveReference(&url.URL{Path: "/api/v1/logout"})

	// 2. Составляем запрос.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.String(), nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 3. Выполняем запрос.
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// 4. Проверяем статус.
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}
