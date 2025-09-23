// Package transport используется для выполнения запросов к серверу.
package transport

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client - обертка над http.Client с базовым URL и токеном.
type Client struct {
	BaseURL *url.URL     // базовый адрес сервера
	HTTP    *http.Client // переиспользуем один клиент
	Token   string
}

// NewClient создает новый клиент.
func NewClient(rawBaseURL string) (*Client, error) {
	if !strings.HasPrefix(rawBaseURL, "http://") && !strings.HasPrefix(rawBaseURL, "https://") {
		return nil, fmt.Errorf("base URL must start with http:// or https://")
	}
	u, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("bad base url: %w", err)
	}
	return &Client{
		BaseURL: u,
		HTTP: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

// SetToken сохраняет токен для последующих запросов.
func (c *Client) SetToken(tok string) {
	c.Token = tok
}

// do — выполняет запрос, автоматически подставляя Cookie при наличии токена.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	// Если у нас есть токен — добавляем Cookie: auth_token=<token>
	if c.Token != "" {
		req.Header.Set("Cookie", "auth_token="+c.Token)
	}
	// Общие заголовки
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	return c.HTTP.Do(req)
}
