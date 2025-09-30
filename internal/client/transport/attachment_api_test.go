package transport_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	commands_attachment "github.com/NailUsmanov/gophkeeper/internal/client/commands/attachment"
	"github.com/NailUsmanov/gophkeeper/internal/client/session"
	"github.com/NailUsmanov/gophkeeper/internal/client/transport"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// ---------- helpers ----------

// root с persistent-флагом --server и деревом attachment
func makeRootWithAttachment() *cobra.Command {
	root := &cobra.Command{Use: "gk"}
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.PersistentFlags().String("server", "", "base URL of the GophKeeper server (can be GK_SERVER_URL)")

	root.AddCommand(commands_attachment.NewAttachmentCmd())
	return root
}

// перехват stdout на время f()
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	f()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

// подготовить HOME и записать токен
func withSessionToken(t *testing.T, token string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, session.SaveToken(token))
	p, err := session.Path()
	require.NoError(t, err)
	_, err = os.Stat(p)
	require.NoError(t, err)
}

// ---------- tests: UPLOAD ----------

func TestAttachmentUpload_Success(t *testing.T) {
	withSessionToken(t, "TOK-UP")

	// временный файл для загрузки
	dir := t.TempDir()
	src := filepath.Join(dir, "demo.txt")
	require.NoError(t, os.WriteFile(src, []byte("hello upload"), 0o644))

	secretID := uuid.New().String()

	// фейковый сервер: POST /api/v1/attachments (multipart/form-data)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/attachments", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		// проверим, что пришла cookie с токеном
		c, err := r.Cookie("auth_token")
		require.NoError(t, err)
		require.Equal(t, "TOK-UP", c.Value)

		// распарсим multipart
		require.NoError(t, r.ParseMultipartForm(32<<20))
		require.Equal(t, secretID, r.Form.Get("secret_id"))

		file, hdr, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()
		require.Equal(t, "demo.txt", hdr.Filename)

		data, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, "hello upload", string(data))

		// ответ сервера — метаданные
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(transport.AttachmentMeta{
			ID:          uuid.New().String(),
			FileName:    hdr.Filename,
			SecretID:    secretID,
			OwnerID:     "u1",
			Size:        int64(len(data)),
			ContentType: "text/plain",
			CreatedAt:   time.Now().UTC(),
		})
	}))
	defer ts.Close()

	root := makeRootWithAttachment()
	root.SetArgs([]string{
		"attachment", "upload",
		"--server", ts.URL,
		"--secret-id", secretID,
		"--file", src,
	})

	out := captureStdout(t, func() {
		require.NoError(t, root.Execute())
	})

	require.Contains(t, out, `"file_name": "demo.txt"`)
	require.Contains(t, out, `"secret_id": "`+secretID+`"`)
}

func TestAttachmentUpload_BadUUID(t *testing.T) {
	withSessionToken(t, "TOK-UP2")

	root := makeRootWithAttachment()
	root.SetArgs([]string{
		"attachment", "upload",
		"--server", "http://example",
		"--secret-id", "not-a-uuid",
		"--file", "any",
	})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--secret-id must be a valid UUID")
}

func TestAttachmentUpload_MissingFile(t *testing.T) {
	withSessionToken(t, "TOK-UP3")

	secretID := uuid.New().String()

	root := makeRootWithAttachment()
	root.SetArgs([]string{
		"attachment", "upload",
		"--server", "http://example",
		"--secret-id", secretID,
		// без --file → cobra сама вернёт ошибку о required flag
	})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), `required flag(s) "file" not set`)
}

// ---------- tests: DOWNLOAD ----------

func TestAttachmentDownload_Success(t *testing.T) {
	withSessionToken(t, "TOK-DL")

	attID := uuid.New().String()
	payload := "downloaded-bytes"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/attachments/"+attID, r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		c, err := r.Cookie("auth_token")
		require.NoError(t, err)
		require.Equal(t, "TOK-DL", c.Value)

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, payload)
	}))
	defer ts.Close()

	dstDir := t.TempDir()
	dst := filepath.Join(dstDir, "out.txt")

	root := makeRootWithAttachment()
	root.SetArgs([]string{
		"attachment", "download", attID,
		"--server", ts.URL,
		"--dest", dst,
	})

	out := captureStdout(t, func() {
		require.NoError(t, root.Execute())
	})
	require.Contains(t, out, "Saved to "+dst)

	// проверим, что файл записан с правильным содержимым
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Equal(t, payload, string(got))
}

func TestAttachmentDownload_BadUUID(t *testing.T) {
	withSessionToken(t, "TOK-DL2")

	root := makeRootWithAttachment()
	root.SetArgs([]string{
		"attachment", "download", "bad-id",
		"--server", "http://example",
		"--dest", "x",
	})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "id must be a valid UUID")
}

// ---------- tests: LIST ----------

func TestAttachmentList_Success(t *testing.T) {
	withSessionToken(t, "TOK-LS")

	secretID := uuid.New().String()
	items := []transport.AttachmentMeta{
		{
			ID:          uuid.New().String(),
			FileName:    "a.txt",
			SecretID:    secretID,
			OwnerID:     "u1",
			Size:        1,
			ContentType: "text/plain",
			CreatedAt:   time.Now().UTC(),
		},
		{
			ID:          uuid.New().String(),
			FileName:    "b.bin",
			SecretID:    secretID,
			OwnerID:     "u1",
			Size:        2,
			ContentType: "application/octet-stream",
			CreatedAt:   time.Now().UTC(),
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/attachments", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		// проверим cookie
		c, err := r.Cookie("auth_token")
		require.NoError(t, err)
		require.Equal(t, "TOK-LS", c.Value)

		// проверим query
		q := r.URL.Query()
		require.Equal(t, secretID, q.Get("secret_id"))
		require.Equal(t, "20", q.Get("limit"))
		// offset может отсутствовать если 0 — это нормально

		_ = json.NewEncoder(w).Encode(struct {
			Items []transport.AttachmentMeta `json:"items"`
			Total int                        `json:"total"`
		}{Items: items, Total: len(items)})
	}))
	defer ts.Close()

	root := makeRootWithAttachment()
	root.SetArgs([]string{
		"attachment", "list",
		"--server", ts.URL,
		"--secret-id", secretID,
	})

	out := captureStdout(t, func() {
		require.NoError(t, root.Execute())
	})

	require.Contains(t, out, `"total"`)
	require.Contains(t, out, `"a.txt"`)
	require.Contains(t, out, `"b.bin"`)
}

func TestAttachmentList_MissingSecretID(t *testing.T) {
	withSessionToken(t, "TOK-LS2")

	root := makeRootWithAttachment()
	root.SetArgs([]string{
		"attachment", "list",
		"--server", "http://example",
		// без --secret-id → required flag
	})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), `required flag(s) "secret-id" not set`)
}

func TestAttachmentList_BadSecretID(t *testing.T) {
	withSessionToken(t, "TOK-LS3")

	root := makeRootWithAttachment()
	root.SetArgs([]string{
		"attachment", "list",
		"--server", "http://example",
		"--secret-id", "not-a-uuid",
	})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--secret-id must be a valid UUID")
}
