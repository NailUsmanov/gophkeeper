// Package fs реализует файловое хранилище (storage) для вложений.
// Сюда сервис attachment будет складывать бинарные данные файлов.
package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AttachmentStorage хранит файлы на диске в указанной папке (baseDir).
type AttachmentStorage struct {
	baseDir string
}

// NewAttachmentStorage создаёт файловое хранилище.
// baseDir — путь к папке, где будут лежать все файлы (например: "./var/attachments").
func NewAttachmentStorage(baseDir string) (*AttachmentStorage, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("baseDir is empty")
	}
	// os.MkdirAll — создаёт папку (и все недостающие родительские папки).
	// 0700: права доступа "только владелец может читать/писать/заходить".
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir baseDir: %w", err)
	}
	return &AttachmentStorage{baseDir: baseDir}, nil
}

// shardPath строит путь до файла по его ID.
// Чтобы не было тысячи файлов в одной папке, создаю подкаталоги по первым символам ID.
// baseDir="attachments", id="abc123..." → attachments/ab/abc123...
func (a *AttachmentStorage) shardPath(id string) string {
	// 1. Создаем начальную папку хх, если id короче 2 символов.
	prefix := "xx"
	if len(id) >= 2 {
		prefix = id[:2]
	}
	// 2. Склеиваем путь из базового, части айди и самого id через filepath.Join.
	return filepath.Join(a.baseDir, prefix, id)
}

// Save сохраняет данные из io.Reader на диск.
// Возвращает количество записанных байт.
func (a *AttachmentStorage) Save(ctx context.Context, attachmentID string, r io.Reader) (int64, error) {
	// 1. Строим путь к файлу
	target := a.shardPath(attachmentID)
	dir := filepath.Dir(target)
	// 2. Создаем директорию для этого файла, если ее нет.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, fmt.Errorf("mkdir shard dir: %w", err)
	}
	// 3. Пишем во временный файл с расширением .part, чтобы избежать поломанных файлов в случае ошибки.
	tmp := target + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open temp file: %w", err)
	}
	// Гарантируем, что временный файл будет удален, если что то пойдет не так.
	defer f.Close()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp)
		}
	}()
	// 4. Копирую весь поток r в файл.
	n, err := io.Copy(f, r)
	if err != nil {
		return n, fmt.Errorf("copy to temp: %w", err)
	}
	// 5. Сбрасываем данные на диск через f.Sync и закрываем файл.
	if err := f.Sync(); err != nil {
		return n, fmt.Errorf("fsync temp: %w", err)
	}
	if err := f.Close(); err != nil {
		return n, fmt.Errorf("close temp: %w", err)
	}
	// 6. Атомарно переименовываю временный файл в целевой.
	// Гарантируется, что файл либо полностью записан, либо его нет.
	if err := os.Rename(tmp, target); err != nil {
		return n, fmt.Errorf("rename temp -> targer: %w", err)
	}
	cleanup = false
	return n, nil
}

// Open достаёт файл по attachmentID и возвращает поток для чтения.
// reader обязан закрыть вызывающий код.
func (a *AttachmentStorage) Open(ctx context.Context, attachmentID string) (io.ReadCloser, error) {
	// Проверим отменен ли контекст.
	select {
	case <-ctx.Done():
		return nil, ctx.Err() // если отменили прямо перед open
	default:
	}
	// 1. Строим путь к файлу из аттачментID.
	path := a.shardPath(attachmentID)
	// 2. Открываем файл на чтение.
	file, err := os.Open(path)
	if err != nil {
		// если файла нет.
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("open: not found: %w", err)
		}
		return nil, fmt.Errorf("open file: %w", err)
	}
	// 3. Возвращаем открытый файл, читающий закроет его после прочтения.
	return file, nil
}

// Delete удаляет файл из стораджа по attachmentID.
// Если файла нет, возвращает nil.
func (a *AttachmentStorage) Delete(ctx context.Context, attachmentID string) error {
	// 1. Проверяем отменен ли контекст.
	select {
	case <-ctx.Done():
		return ctx.Err() // если отменили прямо перед open
	default:
	}
	// 2. Строим путь к файлу из аттачментID.
	path := a.shardPath(attachmentID)
	// 3. Удаляем файл.
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("delete file: %w", err)
	}
	// 4. Удаляем возможный временный файл.
	_ = os.Remove(path + ".part")
	return nil
}

// AttachmentStorage.Ping используется для проверки соединения с БД.
func (a *AttachmentStorage) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Stat(a.baseDir); err != nil {
		return fmt.Errorf("fs ping: %w", err)
	}
	return nil
}
