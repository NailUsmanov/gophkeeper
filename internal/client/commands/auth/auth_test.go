package commands_auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	commands_auth "github.com/NailUsmanov/gophkeeper/internal/client/commands/auth"
	"github.com/NailUsmanov/gophkeeper/internal/client/transport"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func makeRootWith(cmd *cobra.Command) *cobra.Command {
	root := &cobra.Command{Use: "gk"}
	root.PersistentFlags().String("server", "", "base URL of the GophKeeper server (can be GK_SERVER_URL)")
	root.AddCommand(cmd)
	return root
}

func TestLoginCmd_Success(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/login", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		http.SetCookie(w, &http.Cookie{Name: "auth_token", Value: "COOKIE123"})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(transport.UserResponse{
			ID: "u1", Email: "user@example.com",
		})
	}))
	defer ts.Close()

	root := makeRootWith(commands_auth.NewLoginCmd())
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	root.SetArgs([]string{
		"login",
		"--server", ts.URL,
		"--email", "user@example.com",
		"--password", "secret",
	})

	err := root.Execute()
	require.NoError(t, err)
	require.Contains(t, buf.String(), "Logged in as")
}

func TestLoginCmd_MissingEmail(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	root := makeRootWith(commands_auth.NewLoginCmd())
	root.SetArgs([]string{
		"login",
		"--server", "http://example",
		"--password", "secret",
	})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "email is required")
}

func TestRegisterCmd_MissingEmail(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	root := makeRootWith(commands_auth.NewRegisterCmd())
	root.SetArgs([]string{
		"register",
		"--server", "http://example",
		"--password", "secret",
	})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), `required flag(s) "email"`)
}

func TestLogoutCmd_NoSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // нет файла сессии => должно быть падение

	root := makeRootWith(commands_auth.NewLogoutCmd())
	root.SetArgs([]string{
		"logout",
		"--server", "http://example",
	})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no session")
}
