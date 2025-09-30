// Package transport используется для выполнения запросов к серверу.
//
// attachment_api.go используется для загрузки, выгрузки, обновления, файлов секретов пользователя.
package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// AttachmentMeta - то, что возвращает сервер при загрузке.
type AttachmentMeta struct {
	ID          string    `json:"id"`
	FileName    string    `json:"file_name"`
	SecretID    string    `json:"secret_id"`
	OwnerID     string    `json:"owner_id"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
}

// UploadAttachmentRequest - возврат ответа от сервера.
type UploadAttachmentRequest struct {
	SecretID string `json:"secret_id"`
	Filename string `json:"file_name"`
}

// ListAttachmentResponse - возврат ответа от сервера списком.
type ListAttachmentsResponse struct {
	Items []AttachmentMeta `json:"items"`
	Total int              `json:"total"`
}

// UploadAttachment - запрос на сервер для загрузки файла секрета пользователя.
func (c *Client) UploadAttachment(ctx context.Context, secretID string, filePath string) (*AttachmentMeta, error) {
	// URL сервера
	ep := c.BaseURL.ResolveReference(&url.URL{Path: "/api/v1/attachments"})

	// открываем файл
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// Готовим streaming-тело запроса: io.Pipe реализует reader и writer
	// HTTP читает из pr
	// параллельно мы пишем в pw, данные идут через трубу кусками.
	pr, pw := io.Pipe()

	// Создаем multipart.Writer поверх writer.
	// multipart.Writer упаковывает данные в формат multipart/form-data
	// пишет boundary, заголовки части (Content-Disposition, Content-Type и т.п.)
	// потом он записывает тело (байты файла).
	mw := multipart.NewWriter(pw)

	// Пишем multipart-части в отдельной горутине.
	go func() {
		defer func() {
			_ = mw.Close()
			_ = pw.Close()
		}()

		// Текстовое поле секретID
		if err := mw.WriteField("secret_id", secretID); err != nil {
			_ = pw.CloseWithError(fmt.Errorf("write field secret_id: %w", err))
			return
		}

		// Поле самого файла. Cоздаем секцию для файла с именем поля file
		filename := filepath.Base(filePath)
		filePart, err := mw.CreateFormFile("file", filename)
		if err != nil {
			_ = pw.CloseWithError(fmt.Errorf("create form file: %w", err))
			return
		}

		// Копируем байты из файла прямо в multipart-часть стримом
		if _, err := io.Copy(filePart, f); err != nil {
			_ = pw.CloseWithError(fmt.Errorf("copy file data: %w", err))
			return
		}
	}()

	// Создаем запрос для сервера.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.String(), pr)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	// Выполняем запрос.
	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Коды ответа
	switch resp.StatusCode {
	case http.StatusCreated:
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("unauthorized (401) - login required")
	case http.StatusBadRequest:
		return nil, fmt.Errorf("invalid input (400)")
	case http.StatusUnprocessableEntity:
		return nil, fmt.Errorf("validation failed (422)")
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("server error (500)")
	default:
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Декодим JSON ответ с метаданными файла.
	var meta AttachmentMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &meta, nil

}

// DownloadAttachment запрос на сервер для скачивания файла секрета по ID.
func (c *Client) DownloadAttachment(ctx context.Context, attachmentID, destPath string) error {
	// получаем URL сервера.
	ep := c.BaseURL.ResolveReference(&url.URL{
		Path: "/api/v1/attachments/" + url.PathEscape(attachmentID),
	})

	// Создаем запрос для сервера.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.String(), nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	// Выполняем
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Cтатусы кода
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// гарантируем, что директория под файл существует
	if dir := filepath.Dir(destPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	// Создаем локальный файл и копируем тело ответа
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// ListAttachments - запрос на сервер для выдачи файлов секрета пользователя.
func (c *Client) ListAttachments(ctx context.Context, secretID string, limit, offset int) ([]AttachmentMeta, int, error) {
	// Собираем URL запроса
	ep := c.BaseURL.ResolveReference(&url.URL{Path: "/api/v1/attachments"})

	// Query параметры.
	q := url.Values{}
	if secretID != "" {
		q.Set("secret_id", secretID)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	ep.RawQuery = q.Encode()

	// Составляем запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	// Выполняем запрос
	resp, err := c.do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Коды ответа
	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized:
		return nil, 0, fmt.Errorf("unauthorized (401) - login required")
	case http.StatusBadRequest:
		return nil, 0, fmt.Errorf("invalid input (400)")
	case http.StatusInternalServerError:
		return nil, 0, fmt.Errorf("server error (500)")
	default:
		return nil, 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Декодируем ответ
	var lr ListAttachmentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, 0, fmt.Errorf("decode response: %w", err)
	}
	// чтобы не возвращать nil
	if lr.Items == nil {
		lr.Items = []AttachmentMeta{}
	}
	return lr.Items, lr.Total, nil
}
