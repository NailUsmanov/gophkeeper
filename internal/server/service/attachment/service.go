// Package auth содержит интерфейс бизнес-логики AttachmentService,
// интерфейс доступа к данным AttachmentRepository
// и интерфейс взаимодействия с хранилищем AttachmentStorage.
// Хендлеры работают только с AttachmentService,
// а тот внутри вызывает AttachmentRepository и AttachmentStorage для работы с БД.
package attachment

import (
	"context"
	"io"

	"github.com/NailUsmanov/gophkeeper/internal/models"
)

// AttachmentService — «склейка» бизнес-логики по файлам:
// проверка, что секрет принадлежит owner, запись байтов в Storage,
// запись метаданных в Repository, возврат мета и потока для скачивания.
type AttachmentService interface {
	// Upload — принять файл для секрета owner'a.
	// r — поток байтов (из http.Request.Body или multipart.File). Возвращаем метаданные.
	Upload(ctx context.Context, ownerID, secretID, fileName, contentType string, r *io.Reader) (*models.AttachmentMeta, error)
	// Download — отдать файл, если владелец совпадает.
	// Возвращаем метаданные и поток для чтения. Вызывающий обязан закрыть reader.
	Download(ctx context.Context, ownerID, attachmentID string) (meta *models.AttachmentMeta, reader *io.ReadCloser, err error)
}

// AttachmentRepository — «метаданные файла» (связка с секретом, размер, тип, checksum).
type AttachmentRepository interface {
	Insert(ctx context.Context, meta *models.AttachmentMeta) error
	GetByID(ctx context.Context, attachmentID string) (*models.AttachmentMeta, error)
	Delete(ctx context.Context, attachmentID string) error
}

// AttachmentStorage — «куда кладём байты». Потоковая работа: io.Reader / io.ReadCloser.
// Не знаем ничего о метаданных/секрете.
type AttachmentStorage interface {
	Save(ctx context.Context, attachmentID string, r *io.Reader) error
	Open(ctx context.Context, attachmentID string) (*io.ReadCloser, error)
	Delete(ctx context.Context, attachmentID string) error
}
