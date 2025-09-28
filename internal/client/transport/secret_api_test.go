package transport_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/client/transport"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func rCtx() (ctx interface{ Done() <-chan struct{} }) { return nil } // в твоём коде context.Context — подставь context.TODO()

// удобный шорткат:
func ctx() (c interface{ Done() <-chan struct{} }) { return nil }

func TestCreateSecret_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/secrets", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		// проверка Cookie
		require.Equal(t, "auth_token=TOK", r.Header.Get("Cookie"))

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(transport.SecretResponse{
			ID:        uuid.New().String(),
			Type:      "note",
			Title:     "t",
			Data:      map[string]any{"k": "v"},
			Version:   1,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		})
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	cl.SetToken("TOK")

	resp, err := cl.CreateSecret(context.Background(), transport.CreateSecretRequest{
		Type:  "note",
		Title: "t",
		Data:  map[string]any{"k": "v"},
	})
	require.NoError(t, err)
	require.Equal(t, "note", resp.Type)
}

func TestCreateSecret_MarshalError(t *testing.T) {
	cl, _ := transport.NewClient("http://example")
	// json.Marshal не умеет кодировать канал/функцию
	_, err := cl.CreateSecret(context.Background(), transport.CreateSecretRequest{
		Type:  "note",
		Title: "t",
		Data:  map[string]any{"bad": make(chan int)},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "marshal")
}

func TestCreateSecret_StatusErrors_AndUnexpected(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{http.StatusUnauthorized, "unauthorized"},
		{http.StatusBadRequest, "invalid input"},
		{http.StatusUnprocessableEntity, "validation failed"},
		{http.StatusInternalServerError, "server error"},
		{http.StatusTeapot, "unexpected status 418"},
	}
	for _, tc := range cases {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
		}))
		t.Run(tc.want, func(t *testing.T) {
			defer ts.Close()
			cl, _ := transport.NewClient(ts.URL)
			_, err := cl.CreateSecret(context.Background(), transport.CreateSecretRequest{
				Type:  "note",
				Title: "t",
				Data:  map[string]any{"k": "v"},
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestCreateSecret_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":}`)) // ломаный JSON
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	_, err := cl.CreateSecret(context.Background(), transport.CreateSecretRequest{
		Type:  "note",
		Title: "t",
		Data:  map[string]any{"k": "v"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode")
}

func TestGetSecret_Success(t *testing.T) {
	id := uuid.New().String()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/secrets/"+id, r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "auth_token=TOK", r.Header.Get("Cookie"))

		_ = json.NewEncoder(w).Encode(transport.SecretResponse{
			ID:        id,
			Type:      "note",
			Title:     "hello",
			Data:      map[string]any{"note": "world"},
			Version:   1,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		})
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	cl.SetToken("TOK")
	got, err := cl.GetSecret(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, got.ID)
}

func TestGetSecret_StatusErrors_AndUnexpected(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{http.StatusUnauthorized, "unauthorized"},
		{http.StatusNotFound, "not found"},
		{http.StatusInternalServerError, "server error"},
		{http.StatusTeapot, "unexpected status 418"},
	}
	id := uuid.New().String()
	for _, tc := range cases {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
		}))
		t.Run(tc.want, func(t *testing.T) {
			defer ts.Close()
			cl, _ := transport.NewClient(ts.URL)
			_, err := cl.GetSecret(context.Background(), id)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestGetSecret_DecodeError(t *testing.T) {
	id := uuid.New().String()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":}`))
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	_, err := cl.GetSecret(context.Background(), id)
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode")
}

func TestUpdateSecret_Success(t *testing.T) {
	id := uuid.New().String()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/secrets/"+id, r.URL.Path)
		require.Equal(t, http.MethodPut, r.Method)

		_ = json.NewEncoder(w).Encode(transport.SecretResponse{
			ID:        id,
			Type:      "note",
			Title:     "new",
			Data:      map[string]any{"note": "x"},
			Version:   2,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		})
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	resp, err := cl.UpdateSecret(context.Background(), id, transport.UpdateSecretRequest{
		Title:   "new",
		Data:    map[string]any{"note": "x"},
		Version: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, resp.Version)
}
func TestUpdateSecret_MarshalError(t *testing.T) {
	cl, _ := transport.NewClient("http://example")
	_, err := cl.UpdateSecret(context.Background(), "id1", transport.UpdateSecretRequest{
		Title:   "x",
		Data:    map[string]any{"bad": func() {}}, // функции не кодируются
		Version: 1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "marshal")
}

func TestUpdateSecret_StatusErrors_AndUnexpected(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{http.StatusUnauthorized, "unauthorized"},
		{http.StatusBadRequest, "invalid input"},
		{http.StatusConflict, "version conflict"},
		{http.StatusUnprocessableEntity, "validation failed"},
		{http.StatusInternalServerError, "server error"},
		{http.StatusTeapot, "unexpected status 418"},
	}
	for _, tc := range cases {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
		}))
		t.Run(tc.want, func(t *testing.T) {
			defer ts.Close()
			cl, _ := transport.NewClient(ts.URL)
			_, err := cl.UpdateSecret(context.Background(), "id1", transport.UpdateSecretRequest{
				Title:   "x",
				Data:    map[string]any{"k": "v"},
				Version: 1,
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestUpdateSecret_DecodeError(t *testing.T) {
	id := "id1"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// проверим URL (pathEscape/resolve)
		want := "/api/v1/secrets/" + url.PathEscape(id)
		require.Equal(t, want, r.URL.Path)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":`)) // ломаный JSON
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	_, err := cl.UpdateSecret(context.Background(), id, transport.UpdateSecretRequest{
		Title:   "x",
		Data:    map[string]any{"k": "v"},
		Version: 1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode")
}

func TestListSecrets_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/secrets", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		q := r.URL.Query()
		require.Equal(t, "10", q.Get("limit"))

		off := q.Get("offset")
		if off == "" { // клиент не отправляет offset, если он 0 — трактуем как 0
			off = "0"
		}
		require.Equal(t, "0", off)

		_ = json.NewEncoder(w).Encode(transport.ListSecretsResponse{
			Items: []transport.SecretResponse{
				{ID: uuid.New().String(), Type: "note", Title: "A"},
				{ID: uuid.New().String(), Type: "password", Title: "B"},
			},
			Total: 2,
		})
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	items, total, err := cl.ListSecrets(context.Background(), 10, 0, "", time.Time{})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, items, 2)
}

func TestListSecrets_StatusErrors_AndUnexpected(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{http.StatusUnauthorized, "unauthorized"},
		{http.StatusBadRequest, "invalid input"},
		{http.StatusInternalServerError, "server error"},
		{http.StatusTeapot, "unexpected status 418"},
	}
	for _, tc := range cases {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
		}))
		t.Run(tc.want, func(t *testing.T) {
			defer ts.Close()
			cl, _ := transport.NewClient(ts.URL)
			_, _, err := cl.ListSecrets(context.Background(), 10, 0, "", time.Time{})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestListSecrets_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":`)) // ломаный JSON
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	_, _, err := cl.ListSecrets(context.Background(), 10, 0, "", time.Time{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode")
}

func TestListSecrets_QueryParamsAndNilItemsHandled(t *testing.T) {
	typ := "note"
	after := time.Now().UTC().Add(-time.Hour).Round(time.Second)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// проверим корректность query
		q := r.URL.Query()
		require.Equal(t, "5", q.Get("limit"))
		// offset=0 клиент не отправляет — это ок: трактуем как 0
		require.Empty(t, q.Get("offset"))
		require.Equal(t, typ, q.Get("type"))

		gotAfter := q.Get("updated_after")
		// Парсим и сравниваем с погрешностью
		parsed, err := time.Parse(time.RFC3339, gotAfter)
		require.NoError(t, err)
		require.WithinDuration(t, after, parsed, time.Second)

		w.WriteHeader(http.StatusOK)
		// items=null → библиотека должна нормализовать в [] (а не nil)
		_ = json.NewEncoder(w).Encode(transport.ListSecretsResponse{
			Items:  nil,
			Limit:  5,
			Offset: 0,
			Total:  0,
		})
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	items, total, err := cl.ListSecrets(context.Background(), 5, 0, typ, after)
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.NotNil(t, items)
	require.Len(t, items, 0)
}
