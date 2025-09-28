package commands_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/NailUsmanov/gophkeeper/internal/client/commands"
	"github.com/stretchr/testify/require"
)

func capStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

func TestVersionCommand_PrintsBuildInfo(t *testing.T) {
	root := commands.NewRootCmd("1.2.3", "2025-09-10T00:00:00Z", "abc123")

	out := capStdout(func() {
		root.SetArgs([]string{"version"})
		_ = root.Execute()
	})

	require.Contains(t, out, "1.2.3")
	require.Contains(t, out, "2025-09-10T00:00:00Z")
	require.Contains(t, out, "abc123")
}

func TestExecute_NoArgs_ShowsHelp(t *testing.T) {
	root := commands.NewRootCmd("v", "d", "c")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{}) // без сабкоманд

	_ = root.Execute()
	require.Contains(t, buf.String(), "Usage:")
}

func TestRoot_HasServerFlag(t *testing.T) {
	root := commands.NewRootCmd("v", "d", "c")
	f := root.PersistentFlags().Lookup("server")
	require.NotNil(t, f)
	fmt.Println(f.Usage)
}

func TestExecute_RunsVersionSubcommand(t *testing.T) {
	// Подменяем os.Args для cobra
	old := os.Args
	defer func() { os.Args = old }()

	os.Setenv("GK_SERVER_URL", "http://localhost:8080")
	defer os.Unsetenv("GK_SERVER_URL")

	os.Args = []string{"gk", "version", "--server", "http://localhost:8080"}

	err := commands.Execute("1.0.0", "2025-09-10T00:00:00Z", "abcd1234")
	require.NoError(t, err)
}
