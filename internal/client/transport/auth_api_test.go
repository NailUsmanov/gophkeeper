package transport_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NailUsmanov/gophkeeper/internal/client/transport"
	"github.com/stretchr/testify/require"
)

func TestLogin_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/login", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		http.SetCookie(w, &http.Cookie{Name: "auth_token", Value: "COOKIE123"})
		_ = json.NewEncoder(w).Encode(transport.UserResponse{
			ID: "u1", Email: "user@example.com",
		})
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	user, tok, err := cl.Login(context.Background(), "user@example.com", "pass")
	require.NoError(t, err)
	require.Equal(t, "COOKIE123", tok)
	require.Equal(t, "u1", user.ID)
}

func TestRegister_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/register", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		http.SetCookie(w, &http.Cookie{Name: "auth_token", Value: "C2"})
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(transport.UserResponse{ID: "u2", Email: "e@x"})
	}))
	defer ts.Close()

	cl, _ := transport.NewClient(ts.URL)
	u, tok, err := cl.Register(context.Background(), "e@x", "p")
	require.NoError(t, err)
	require.Equal(t, "C2", tok)
	require.Equal(t, "u2", u.ID)
}
