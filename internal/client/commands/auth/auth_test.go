package commands_auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	commands_auth "github.com/NailUsmanov/gophkeeper/internal/client/commands/auth"
	"github.com/NailUsmanov/gophkeeper/internal/client/session"
	"github.com/NailUsmanov/gophkeeper/internal/client/transport"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// запускает корневую команду с заданными аргументами через ExecuteContext,
// возвращает stdout/stderr и ошибку.
func execRootWithCtx(root *cobra.Command, args ...string) (string, error) {
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	return buf.String(), root.ExecuteContext(context.Background())
}

func captureStdout(t *testing.T, f func() error) (string, error) {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w
	var buf bytes.Buffer

	done := make(chan struct{})

	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	callErr := f() // выполняем команду внутри перехвата stdout

	_ = w.Close()
	<-done
	os.Stdout = orig
	return buf.String(), callErr
}

func makeRootWith(cmd *cobra.Command) *cobra.Command {
	root := &cobra.Command{Use: "gk"}
	root.PersistentFlags().String("server", "", "base URL of the GophKeeper server (can be GK_SERVER_URL)")
	root.AddCommand(cmd)
	return root
}

func withSessionToken(t *testing.T, tok string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, session.SaveToken(tok))
	p, err := session.Path()
	require.NoError(t, err)
	_, err = os.Stat(p)
	require.NoError(t, err) // файл действительно есть
	return p
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

func runCmd(cmd *cobra.Command, args ...string) (string, error) {
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	return buf.String(), cmd.Execute()
}

func TestNewLogoutCmd_Help(t *testing.T) {
	root := makeRootWith(commands_auth.NewLogoutCmd())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"logout", "--help"})
	err := root.Execute()
	require.NoError(t, err)
	require.Contains(t, buf.String(), "Usage:")
}

func TestNewRegisterCmd_RequiresFlags(t *testing.T) {
	cmd := commands_auth.NewRegisterCmd()
	_, err := runCmd(cmd) // без флагов
	require.Error(t, err)
}

func TestNewLogoutCmd_Success_RemovesSession(t *testing.T) {
	const tok = "TOK-OK"
	p := withSessionToken(t, tok)

	// Фейковый сервер logout -> 204
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/logout", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusNoContent) // ожидаемый код
	}))
	defer ts.Close()

	root := makeRootWith(commands_auth.NewLogoutCmd())

	// Ловим вывод команды через writer Cobra
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"logout", "--server", ts.URL})

	err := root.ExecuteContext(context.Background())
	require.NoError(t, err)

	out := &buf
	require.Contains(t, out.String(), "Logged out successfully.")

	// Файл сессии должен быть удалён
	_, statErr := os.Stat(p)
	require.Error(t, statErr)
	require.True(t, os.IsNotExist(statErr))
}
func TestNewLogoutCmd_ServerFails_ButSessionCleared(t *testing.T) {
	const tok = "TOK-FAIL"
	p := withSessionToken(t, tok)

	// сервер вернёт 500 → команда должна вывести warning, но всё равно удалить локальную сессию
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/logout", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		_, _ = r.Cookie("auth_token")
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer ts.Close()

	root := makeRootWith(commands_auth.NewLogoutCmd())

	out, err := captureStdout(t, func() error {
		// направляем вывод Cobra в stdout, который мы перехватываем
		root.SetOut(os.Stdout)
		root.SetErr(os.Stdout)
		root.SetArgs([]string{"logout", "--server", ts.URL})
		return root.ExecuteContext(context.Background())
	})
	require.NoError(t, err) // команда не должна падать

	// проверяем обе строки из одного потока
	require.Contains(t, out, "warning: logout request failed")
	require.Contains(t, out, "Logged out successfully.")

	// файл сессии удалён
	_, statErr := os.Stat(p)
	require.Error(t, statErr)
	require.True(t, os.IsNotExist(statErr))
}

func TestNewLogoutCmd_UsesDefaultServer_WhenFlagMissing(t *testing.T) {
	p := withSessionToken(t, "TOK-DEFAULT")

	root := makeRootWith(commands_auth.NewLogoutCmd())

	out, err := captureStdout(t, func() error {
		// направляем вывод Cobra туда же, куда и обычный fmt.Println
		root.SetOut(os.Stdout)
		root.SetErr(os.Stdout)
		root.SetArgs([]string{"logout"}) // без --server
		return root.ExecuteContext(context.Background())
	})
	require.NoError(t, err)

	// проверяем итоговый вывод
	require.Contains(t, out, "Logged out successfully.")

	// локальная сессия должна быть удалена
	_, statErr := os.Stat(p)
	require.Error(t, statErr)
	require.True(t, os.IsNotExist(statErr))
}

// Дополнительно: sanity-тест на корректность таймаута (контекст не падает раньше времени).
// Сетевой вызов просто быстро ответит; нам важно, что команда не «зависает» более разумного срока.
func TestNewLogoutCmd_ContextTimeoutIsReasonable(t *testing.T) {
	_ = withSessionToken(t, "TOK-TIMEOUT")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	root := makeRootWith(commands_auth.NewLogoutCmd())
	_, err := runCmd(root, "logout", "--server", ts.URL)
	require.NoError(t, err)
}
