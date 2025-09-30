package commands_secret_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	commands_secret "github.com/NailUsmanov/gophkeeper/internal/client/commands/secret"
	"github.com/NailUsmanov/gophkeeper/internal/client/session"
	"github.com/NailUsmanov/gophkeeper/internal/client/transport"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// --- helpers ---------------------------------------------------------------

// корневая команда с persistent-флагом --server и поддеревом "secret"
func makeRootWithSecret() *cobra.Command {
	root := &cobra.Command{Use: "gk"}
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.PersistentFlags().String("server", "", "base URL of the GophKeeper server (can be GK_SERVER_URL)")

	secret := &cobra.Command{Use: "secret"}
	secret.AddCommand(
		commands_secret.NewCreateSecret(),
		commands_secret.NewGetSecret(),
		commands_secret.NewListSecrets(),
		commands_secret.NewUpdateSecret(),
	)
	root.AddCommand(secret)
	return root
}

// перехват stdout на время выполнения f()
func captureStdout(t *testing.T, f func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = orig
	}()

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	f()
	_ = w.Close() // завершить io.Copy
	out := <-done
	return out
}

// подготовить HOME и записать токен
func withSessionToken(t *testing.T, token string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, session.SaveToken(token))
	// убедимся, что файл реально появился (диагностика на будущее)
	p, err := session.Path()
	require.NoError(t, err)
	_, err = os.Stat(p)
	require.NoError(t, err)
}

// --- tests -----------------------------------------------------------------

func TestSecretCreate_Success(t *testing.T) {
	withSessionToken(t, "TOK123")

	// фейковый сервер: принимает POST /api/v1/secrets
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/secrets", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		// проверим Cookie с токеном
		c, err := r.Cookie("auth_token")
		require.NoError(t, err)
		require.Equal(t, "TOK123", c.Value)

		var req transport.CreateSecretRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "note", req.Type)
		require.Equal(t, "My first secret", req.Title)
		require.Equal(t, "hello", req.Data["note"])

		// ответим 201 с телом секрета
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(transport.SecretResponse{
			ID:        uuid.New().String(),
			Type:      "note",
			Title:     req.Title,
			Data:      req.Data,
			Version:   1,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		})
	}))
	defer ts.Close()

	root := makeRootWithSecret()
	root.SetArgs([]string{
		"secret", "create",
		"--server", ts.URL,
		"--type", "note",
		"--title", "My first secret",
		"--data", `{"note":"hello"}`,
	})

	out := captureStdout(t, func() {
		require.NoError(t, root.Execute())
	})

	require.Contains(t, out, `"type": "note"`)
	require.Contains(t, out, `"title": "My first secret"`)
}

func TestSecretCreate_BadType(t *testing.T) {
	withSessionToken(t, "TOK123")

	// сервер даже не понадобится — ошибка валидируется в PreRunE
	root := makeRootWithSecret()
	root.SetArgs([]string{
		"secret", "create",
		"--server", "http://example",
		"--type", "text", // неразрешённый тип по твоей валидации
		"--title", "X",
	})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--type must be one of: password|note|card|file")
}

func TestSecretGet_Success(t *testing.T) {
	withSessionToken(t, "TOK456")
	id := uuid.New().String()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/secrets/"+id, r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		c, err := r.Cookie("auth_token")
		require.NoError(t, err)
		require.Equal(t, "TOK456", c.Value)

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

	root := makeRootWithSecret()
	root.SetArgs([]string{
		"secret", "get", id,
		"--server", ts.URL,
	})

	out := captureStdout(t, func() {
		require.NoError(t, root.Execute())
	})

	require.Contains(t, out, `"id": "`+id+`"`)
	require.Contains(t, out, `"type": "note"`)
}

func TestSecretGet_BadUUID(t *testing.T) {
	withSessionToken(t, "TOK")
	root := makeRootWithSecret()
	root.SetArgs([]string{"secret", "get", "not-a-uuid", "--server", "http://example"})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "id must be a valid UUID")
}

func TestSecretList_Success(t *testing.T) {
	withSessionToken(t, "TOK789")

	items := []transport.SecretResponse{
		{
			ID:        uuid.New().String(),
			Type:      "note",
			Title:     "A",
			Data:      map[string]any{"note": "a"},
			Version:   1,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		{
			ID:        uuid.New().String(),
			Type:      "password",
			Title:     "B",
			Data:      map[string]any{"login": "u"},
			Version:   2,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/secrets", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		// проверим query — по умолчанию limit=20, offset=0 (как в твоём коде)
		q := r.URL.Query()
		require.Equal(t, "20", q.Get("limit")) // клиент всегда шлёт limit>0 → попадает в URL
		off := q.Get("offset")
		if off == "" { // твой клиент не добавляет offset, если он 0
			off = "0"
		}
		require.Equal(t, "0", off)

		_ = json.NewEncoder(w).Encode(struct {
			Items []transport.SecretResponse `json:"items"`
			Total int                        `json:"total"`
		}{Items: items, Total: len(items)})
	}))
	defer ts.Close()

	root := makeRootWithSecret()
	root.SetArgs([]string{
		"secret", "list",
		"--server", ts.URL,
	})

	out := captureStdout(t, func() {
		require.NoError(t, root.Execute())
	})

	require.Contains(t, out, `"total"`) // печатается массив, проверим хотя бы заголовки
	require.Contains(t, out, items[0].ID[:8])
	require.Contains(t, out, items[1].ID[:8])
}

func TestSecretList_BadType(t *testing.T) {
	withSessionToken(t, "TOK")
	root := makeRootWithSecret()
	root.SetArgs([]string{
		"secret", "list",
		"--server", "http://example",
		"--type", "text", // невалидный тип
	})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--type must be one of: note|password|card|file")
}

func TestSecretUpdate_Success(t *testing.T) {
	withSessionToken(t, "TOK999")
	id := uuid.New().String()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/secrets/"+id, r.URL.Path)
		require.Equal(t, http.MethodPut, r.Method)

		var req transport.UpdateSecretRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, 1, req.Version)
		require.Equal(t, "Second title", req.Title)
		require.Equal(t, "new note", req.Data["note"])

		_ = json.NewEncoder(w).Encode(transport.SecretResponse{
			ID:        id,
			Type:      "note",
			Title:     req.Title,
			Data:      req.Data,
			Version:   2,
			CreatedAt: time.Now().UTC().Add(-time.Hour),
			UpdatedAt: time.Now().UTC(),
		})
	}))
	defer ts.Close()

	root := makeRootWithSecret()
	root.SetArgs([]string{
		"secret", "update", id,
		"--server", ts.URL,
		"--version", "1",
		"--title", "Second title",
		"--data", `{"note":"new note"}`,
	})

	out := captureStdout(t, func() {
		require.NoError(t, root.Execute())
	})

	require.Contains(t, out, `"version": 2`)
	require.Contains(t, out, `"Second title"`)
}

func TestSecretUpdate_MissingVersion(t *testing.T) {
	withSessionToken(t, "TOK")
	id := uuid.New().String()

	root := makeRootWithSecret()
	root.SetArgs([]string{
		"secret", "update", id,
		"--server", "http://example",
		// без --version
	})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), `required flag(s) "version" not set`)
}
func TestNewSecretCmd_Basics(t *testing.T) {
	cmd := commands_secret.NewSecretCmd()

	require.Equal(t, "secret", cmd.Use)
	require.Contains(t, cmd.Short, "Manage")

	// соберём имена подкоманд
	names := []string{}
	for _, c := range cmd.Commands() {
		names = append(names, c.Use)
	}

	// правильные Use для подкоманд
	require.ElementsMatch(t,
		[]string{"create", "get <id>", "list", "update <id>"},
		names,
	)
}
