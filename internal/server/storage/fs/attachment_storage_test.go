package fs

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewAttachmentStorage_OK(t *testing.T) {
	tmp := t.TempDir()
	st, err := NewAttachmentStorage(filepath.Join(tmp, "attachments"))
	if err != nil {
		t.Fatalf("NewAttachmentStorage: %v", err)
	}
	// директория должна быть создана
	if _, err := os.Stat(st.baseDir); err != nil {
		t.Fatalf("baseDir not created: %v", err)
	}
}

func TestNewAttachmentStorage_EmptyBaseDir(t *testing.T) {
	if _, err := NewAttachmentStorage(""); err == nil {
		t.Fatalf("expected error on empty baseDir")
	}
}

func TestShardPath(t *testing.T) {
	tmp := t.TempDir()
	st, _ := NewAttachmentStorage(tmp)

	tests := []struct {
		id   string
		want string
	}{
		{"ab123", filepath.Join(tmp, "ab", "ab123")},
		{"a", filepath.Join(tmp, "xx", "a")}, // короче 2 символов → "xx"
		{"", filepath.Join(tmp, "xx", "")},
	}
	for _, tc := range tests {
		got := st.shardPath(tc.id)
		if got != tc.want {
			t.Fatalf("shardPath(%q) = %q; want %q", tc.id, got, tc.want)
		}
	}
}

func TestSaveOpenDelete_OK(t *testing.T) {
	tmp := t.TempDir()
	st, _ := NewAttachmentStorage(tmp)

	ctx := context.Background()
	id := "ab123456"
	content := []byte("hello world")

	// Save
	n, err := st.Save(ctx, id, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if n != int64(len(content)) {
		t.Fatalf("Save wrote %d bytes; want %d", n, len(content))
	}
	target := st.shardPath(id)
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("file not found after Save: %v", err)
	}

	// Open
	rc, err := st.Open(ctx, id)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, content) {
		t.Fatalf("Open content mismatch: got=%q want=%q", got, content)
	}

	// Delete
	if err := st.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("file still exists after Delete")
	}

	// Delete again → idempotent
	if err := st.Delete(ctx, id); err != nil {
		t.Fatalf("Delete second time should be nil, got: %v", err)
	}
}

func TestOpen_NotFound(t *testing.T) {
	tmp := t.TempDir()
	st, _ := NewAttachmentStorage(tmp)

	if _, err := st.Open(context.Background(), "nope"); err == nil {
		t.Fatalf("Open expected error for missing file")
	}
}

func TestPing_OK_and_Fail(t *testing.T) {
	tmp := t.TempDir()
	st, _ := NewAttachmentStorage(filepath.Join(tmp, "attachments"))

	// OK
	if err := st.Ping(context.Background()); err != nil {
		t.Fatalf("Ping ok: %v", err)
	}

	// Удалим базовую директорию — Ping должен упасть
	if err := os.RemoveAll(st.baseDir); err != nil {
		t.Fatalf("remove baseDir: %v", err)
	}
	if err := st.Ping(context.Background()); err == nil {
		t.Fatalf("Ping expected error after baseDir removed")
	}
}

func TestOpen_Delete_ContextCanceled(t *testing.T) {
	tmp := t.TempDir()
	st, _ := NewAttachmentStorage(tmp)

	// Подготовим файл
	_, _ = st.Save(context.Background(), "zz1", bytes.NewReader([]byte("x")))

	// Отменённый контекст
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Open с отменённым контекстом
	if _, err := st.Open(ctx, "zz1"); err == nil {
		t.Fatalf("Open with canceled ctx: expected error")
	}

	// Delete с отменённым контекстом
	if err := st.Delete(ctx, "zz1"); err == nil {
		t.Fatalf("Delete with canceled ctx: expected error")
	}
}

func TestSave_CreatesShardDirs(t *testing.T) {
	tmp := t.TempDir()
	st, _ := NewAttachmentStorage(tmp)

	id := "xy987"
	target := st.shardPath(id)
	shardDir := filepath.Dir(target)

	if _, err := os.Stat(shardDir); !os.IsNotExist(err) {
		t.Fatalf("shard dir should not exist yet")
	}
	if _, err := st.Save(context.Background(), id, bytes.NewReader([]byte("data"))); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if fi, err := os.Stat(shardDir); err != nil || !fi.IsDir() {
		t.Fatalf("shard dir not created")
	}
}

func TestOpen_WhenCreatedLater(t *testing.T) {
	// Проверяем, что открытие после небольшой задержки всё ок (реалистичный сценарий)
	tmp := t.TempDir()
	st, _ := NewAttachmentStorage(tmp)

	id := "cd42"
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = st.Save(context.Background(), id, bytes.NewReader([]byte("late")))
	}()

	// подождём и откроем
	time.Sleep(100 * time.Millisecond)
	rc, err := st.Open(context.Background(), id)
	if err != nil {
		t.Fatalf("Open after delayed Save: %v", err)
	}
	rc.Close()
}
