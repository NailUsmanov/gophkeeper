package middlewares

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/security/token"
	"github.com/NailUsmanov/gophkeeper/internal/server/storage/session/memory"
	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGzipMiddleware_ResponseCompression(t *testing.T) {
	r := chi.NewRouter()
	r.Use(GzipMiddleware)
	r.Get("/big", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// достаточно большой ответ, чтобы точно прошёл через Write
		io.WriteString(w, strings.Repeat("hello world ", 1000))
	})

	req := httptest.NewRequest(http.MethodGet, "/big", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

	// проверим, что тело реально gzipped и разжимается
	gr, err := gzip.NewReader(bytes.NewReader(rr.Body.Bytes()))
	require.NoError(t, err)
	defer gr.Close()
	body, err := io.ReadAll(gr)
	require.NoError(t, err)
	require.Contains(t, string(body), "hello world")
}

func TestNewCompressReader_Read(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte("payload"))
	_ = zw.Close()

	cr, _ := NewCompressReader(io.NopCloser(bytes.NewReader(buf.Bytes())))
	defer cr.Close()

	out := make([]byte, 4)
	n, err := cr.Read(out)
	require.NoError(t, err)
	require.Equal(t, 4, n)
	// дочитываем остаток
	rest, err := io.ReadAll(cr)
	require.NoError(t, err)
	require.Equal(t, "oad", string(rest))
}
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

func TestAuthMiddleware(t *testing.T) {
	store := memory.NewStore()
	tm := token.NewOpaqueManager(store, time.Hour)
	tok, _ := tm.Issue(context.Background(), "u-ctx")

	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(UserLoginKey).(string)
		require.Equal(t, "u-ctx", uid)
		w.WriteHeader(http.StatusOK)
	})

	h := AuthMiddleWare(tm)(protected)

	// Unauthorized
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)

	// Authorized
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.AddCookie(&http.Cookie{Name: "auth_token", Value: tok})
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	require.Equal(t, http.StatusOK, rr2.Code)
}
