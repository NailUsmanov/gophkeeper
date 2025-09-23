// Package transport используется для выполнения запросов к серверу.
//
// secret_api используется для запросов создания, удаления, получения, обновления секретов к серверу.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// CreateSecretRequest — тело POST /api/v1/secrets.
type CreateSecretRequest struct {
	Type  string                 `json:"type"`  // "password" | "note" | "card" | "file"
	Title string                 `json:"title"` // заголовок
	Data  map[string]interface{} `json:"data"`  // произвольные данные по типу
}

// SecretResponse — ответ сервера на создание/чтение секрета.
type SecretResponse struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Data      map[string]interface{} `json:"data"`
	Version   int                    `json:"version"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// ListSecretsResponse - ответ сервера на создание списка секретов.
type ListSecretsResponse struct {
	Items  []SecretResponse `json:"items"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
	Total  int              `json:"total"`
}

// UpdateSecretRequest - тело POST /api/v1/secrets/update
type UpdateSecretRequest struct {
	Title   string                 `json:"title,omitempty"` // заголовок
	Data    map[string]interface{} `json:"data,omitempty"`  // произвольные данные по типу
	Version int                    `json:"version"`         // версия обновления
}

// CreateSecret отправляет POST /api/v1/secrets.
// Требует установленный токен (cookie)
func (c *Client) CreateSecret(ctx context.Context, req CreateSecretRequest) (*SecretResponse, error) {
	// 1) URL
	ep := c.BaseURL.ResolveReference(&url.URL{Path: "/api/v1/secrets"})
	// 2) JSON body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal secret request: %w", err)
	}
	// 3) HTTP-запрос
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// 4) Выполняем
	resp, err := c.do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	// 5) Коды ответа
	switch resp.StatusCode {
	case http.StatusCreated:
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("unauthorized (401) - login required")
	case http.StatusBadRequest:
		return nil, fmt.Errorf("invalid input (400)")
	case http.StatusUnprocessableEntity:
		return nil, fmt.Errorf("validation failed (422)")
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("server error (500)")
	default:
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	// 6) Декодим успешный ответ
	var secret SecretResponse
	if err := json.NewDecoder(resp.Body).Decode(&secret); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &secret, nil
}

// GetSecret - отправляет запрос с получением секрета по id.
func (c *Client) GetSecret(ctx context.Context, id string) (*SecretResponse, error) {
	// URL
	ep := c.BaseURL.ResolveReference(&url.URL{Path: "/api/v1/secrets/" + url.PathEscape(id)})

	// HTTP - запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	// Выполняем  запрос
	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Коды ответа
	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("unauthorized (401) - login required")
	case http.StatusNotFound:
		return nil, fmt.Errorf("not found (404)")
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("server error (500)")
	default:
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Декодим успешный ответ
	var s SecretResponse
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	return &s, nil
}

func (c *Client) ListSecrets(ctx context.Context, limit, offset int, typ string, updatedAfter time.Time) ([]SecretResponse, int, error) {
	// Собираем URL: /api/v1/secrets/{id}
	ep := c.BaseURL.ResolveReference(&url.URL{Path: "/api/v1/secrets"})

	// Query параметры
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	if typ != "" {
		q.Set("type", typ)
	}
	if !updatedAfter.IsZero() {
		q.Set("updated_after", updatedAfter.Format(time.RFC3339))
	}
	ep.RawQuery = q.Encode()

	// HTTP-запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	// Выполняем запрос
	resp, err := c.do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Коды ответа
	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized:
		return nil, 0, fmt.Errorf("unauthorized (401) - login required")
	case http.StatusBadRequest:
		return nil, 0, fmt.Errorf("invalid input (400)")
	case http.StatusInternalServerError:
		return nil, 0, fmt.Errorf("server error (500)")
	default:
		return nil, 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Декодим успешный ответ
	var lr ListSecretsResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, 0, fmt.Errorf("decode resp: %w", err)
	}
	if lr.Items == nil {
		lr.Items = []SecretResponse{}
	}
	return lr.Items, lr.Total, nil
}

func (c *Client) UpdateSecret(ctx context.Context, id string, req UpdateSecretRequest) (*SecretResponse, error) {
	// 1) URL
	ep := c.BaseURL.ResolveReference(&url.URL{Path: "/api/v1/secrets/" + url.PathEscape(id)})
	// 2) JSON body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal secret request: %w", err)
	}
	// 3) HTTP-запрос
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, ep.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// 4) Выполняем
	resp, err := c.do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	// 5) Коды ответа
	switch resp.StatusCode {
	case http.StatusOK:
		// всё ок
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("unauthorized (401) - login required")
	case http.StatusBadRequest:
		return nil, fmt.Errorf("invalid input (400)")
	case http.StatusConflict:
		return nil, fmt.Errorf("version conflict (409)")
	case http.StatusUnprocessableEntity:
		return nil, fmt.Errorf("validation failed (422)")
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("server error (500)")
	default:
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	// 6) Декодим успешный ответ
	var secret SecretResponse
	if err := json.NewDecoder(resp.Body).Decode(&secret); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &secret, nil
}
