package middlewares

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestGzipMiddleWare(t *testing.T) {
	test := struct {
		name            string
		contentEncoding string
		contentType     string
		body            string
		wantStatus      int
	}{
		name:            "gzip json request",
		contentEncoding: "gzip",
		contentType:     "application/json",
		body:            `{"login":"example123"}`,
		wantStatus:      http.StatusCreated,
	}
	t.Run(test.name, func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		gw.Write([]byte(test.body))
		gw.Close()

		req := httptest.NewRequest("POST", "/api/user/login", &buf)
		req.Header.Set("Content-Encoding", test.contentEncoding)
		req.Header.Set("Content-Type", test.contentType)
		req.Header.Set("Accept-Encoding", "gzip")

		// Тестовый обработчик
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
		})

		rec := httptest.NewRecorder()
		GzipMiddleware(handler).ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		assert.Equal(t, test.wantStatus, res.StatusCode)
		assert.Equal(t, "gzip", res.Header.Get("Content-Encoding"))
		assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
	})
}

func TestLoggingMiddleware(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sugar := logger.Sugar()

	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})
	// Создаем middleware с тестовым логгером
	handler := LoggingMiddleware(sugar)(mockHandler)

	t.Run("logs request details", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)
	})
}
