package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NailUsmanov/gophkeeper/internal/client/session"
	"github.com/stretchr/testify/require"
)

// подменим UserConfigDir, чтобы хранение шло в TempDir
func withTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := session.UserConfigDir
	session.UserConfigDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { session.UserConfigDir = old })
	return dir
}

// ------------------------ ТЕСТЫ ------------------------

func TestSaveAndLoadToken(t *testing.T) {
	withTempConfigDir(t)

	token := "TEST-123"
	require.NoError(t, session.SaveToken(token))

	got, err := session.LoadToken()
	require.NoError(t, err)
	require.Equal(t, token, got)
}

func TestSaveToken_Empty(t *testing.T) {
	withTempConfigDir(t)

	err := session.SaveToken("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty token")
}

func TestLoadToken_NoFile(t *testing.T) {
	withTempConfigDir(t)

	got, err := session.LoadToken()
	require.NoError(t, err)
	require.Equal(t, "", got)
}

func TestClear_RemovesFile(t *testing.T) {
	withTempConfigDir(t)

	require.NoError(t, session.SaveToken("X"))
	require.NoError(t, session.Clear())

	// После Clear файла нет
	got, err := session.LoadToken()
	require.NoError(t, err)
	require.Equal(t, "", got)

	// Повторный Clear не падает
	require.NoError(t, session.Clear())
}

func TestLoadToken_BadJSON(t *testing.T) {
	_ = withTempConfigDir(t)

	// Запишем битый JSON
	p, _ := session.Path()
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
	require.NoError(t, os.WriteFile(p, []byte("{bad json"), 0o600))

	_, err := session.LoadToken()
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode")
}
