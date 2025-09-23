package transport_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
