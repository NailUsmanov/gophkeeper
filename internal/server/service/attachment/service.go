// Package attachment содержит интерфейс бизнес-логики AttachmentService,
// интерфейс доступа к данным AttachmentRepository
// и интерфейс взаимодействия с хранилищем AttachmentStorage.
// Хендлеры работают только с AttachmentService,
// а тот внутри вызывает AttachmentRepository и AttachmentStorage для работы с БД.
package attachment

import (
	"context"
	"io"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AttachmentRepository — «метаданные файла» (связка с секретом, размер, тип, checksum).
type AttachmentRepository interface {
	Insert(ctx context.Context, meta *models.AttachmentMeta) error
	GetByID(ctx context.Context, attachmentID string) (*models.AttachmentMeta, error)
	Delete(ctx context.Context, attachmentID string) error
	List(ctx context.Context, ownerID, secretID string, limit, offset int) ([]models.AttachmentMeta, int, error)
	Ping(ctx context.Context) error
}

// AttachmentStorage — «куда кладём байты». Потоковая работа: io.Reader / io.ReadCloser.
// Не знаем ничего о метаданных/секрете.
type AttachmentStorage interface {
	Save(ctx context.Context, attachmentID string, r io.Reader) (int64, error)
	Open(ctx context.Context, attachmentID string) (io.ReadCloser, error)
	Delete(ctx context.Context, attachmentID string) error
	Ping(ctx context.Context) error
}

// AttachmentService сущность для использования хендлерами с репозиторием, стораджем и логером файлов.
type AttachmentService struct {
	repo    AttachmentRepository
	storage AttachmentStorage
	logger  *zap.SugaredLogger
}

// NewAttachmentService конструктор для AttachmentService.
func NewAttachmentService(repo AttachmentRepository, storage AttachmentStorage, logger *zap.SugaredLogger) *AttachmentService {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &AttachmentService{repo: repo, storage: storage, logger: logger}
}

// Upload — заливает бинарь в хранилище байтов файла и создаёт метаданные в БД.
func (a *AttachmentService) Upload(ctx context.Context, ownerID, secretID, filename, contentType string, r io.Reader) (*models.AttachmentMeta, error) {
	// 1. Базовая валидация
	if ownerID == "" || secretID == "" {
		return nil, models.NewBadRequest(map[string]any{"ownerID/secretID": "required"})
	}
	if filename == "" {
		return nil, models.NewBadRequest(map[string]any{"filename": "required"})
	}
	if r == nil {
		return nil, models.NewBadRequest(map[string]any{"file": "nil reader"})
	}
	// 2. Генерация attachmentID.
	attachmentID := generateID()
	// 3. Ограничиваем входной поток, чтобы не уронить сервис слишком большим файлом
	const maxSize int64 = 20 << 20 // 20 MiB — подбери под задачу
	lr := &io.LimitedReader{R: r, N: maxSize + 1}
	// 4. Пишем поток в хранилище через Save.
	n, err := a.storage.Save(ctx, attachmentID, lr)
	if err != nil {
		return nil, models.NewInternal(map[string]any{"storage.Save": err.Error()})
	}
	// Превышение лимита: LimitedReader выдаст чтение > maxSize
	if lr.N == 0 {
		// файл больше допустимого
		_ = a.storage.Delete(ctx, attachmentID)
		return nil, models.NewBadRequest(map[string]any{"size": "file too large"})
	}
	if n <= 0 {
		_ = a.storage.Delete(ctx, attachmentID)
		return nil, models.NewBadRequest(map[string]any{"size": "empty file"})
	}
	// 5. Собираем метаданные.
	meta := models.AttachmentMeta{
		ID:          attachmentID,
		FileName:    filename,
		OwnerID:     ownerID,
		SecretID:    secretID,
		Size:        n,
		ContentType: contentType,
		CreatedAt:   time.Now().UTC(),
	}
	// 6. Сохраняем метаданные в БД.
	if err := a.repo.Insert(ctx, &meta); err != nil {
		// Если БД упала, а в хранилище файл записан — чистим хранилище, чтобы не плодить «сирот».
		a.logger.Errorw("failed to insert attachment metadata",
			"attachmentID", attachmentID,
			"ownerID", ownerID,
			"secretID", secretID,
			"error", err,
		)
		_ = a.storage.Delete(ctx, attachmentID)
		return nil, models.NewInternal(map[string]any{"repo.Insert": err.Error()})
	}
	return &meta, nil
}

// Download — возвращает метаданные и поток чтения файла из хранилища.
func (a *AttachmentService) Download(ctx context.Context, ownerID, attachmentID string) (meta *models.AttachmentMeta, reader io.ReadCloser, err error) {
	// 1. Проверяем, что ownerID и attachmentID не пустые.
	if ownerID == "" || attachmentID == "" {
		return nil, nil, models.NewBadRequest(map[string]any{"ownerID/attachmentID": "required"})
	}
	// 2. Достаем метаданные из репозитория.
	meta, err = a.repo.GetByID(ctx, attachmentID)
	if err != nil {
		return nil, nil, err
	}
	// 3. Проверяем привязку к пользователю.
	if meta.OwnerID != ownerID {
		return nil, nil, models.NewForbidden(map[string]any{"attachmentID": attachmentID})
	}
	// 4. Открываем поток чтения в хранилище и возвращаем ответ.
	reader, err = a.storage.Open(ctx, attachmentID)
	if err != nil {
		a.logger.Errorw("failed to open attachment blob",
			"attachmentID", attachmentID,
			"error", err,
		)
		return meta, nil, models.NewInternal(map[string]any{"storage.Open": err.Error()})
	}
	return meta, reader, nil
}

// List — возвращает список метаданных вложений по secretID с пагинацией.
func (a *AttachmentService) List(ctx context.Context, ownerID, secretID string, limit, offset int) ([]models.AttachmentMeta, int, error) {
	// 1. Валидируем входные данные.
	if ownerID == "" || secretID == "" {
		return nil, 0, models.NewBadRequest(map[string]any{"ownerID/secretID": "required"})
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	// 2. Вызываю репозиторий.
	items, total, err := a.repo.List(ctx, ownerID, secretID, limit, offset)
	if err != nil {
		return nil, 0, models.NewInternal(map[string]any{"repo.List": err.Error()})
	}
	// 3. Возвращаю результат.
	return items, total, nil
}

func generateID() string {
	return uuid.New().String()
}

// Ping - проверяет работает ли БД.
func (a *AttachmentService) Ping(ctx context.Context) error {
	// проверяем базу
	if err := a.repo.Ping(ctx); err != nil {
		return err
	}
	// проверяем сторадж
	if err := a.storage.Ping(ctx); err != nil {
		return err
	}
	return nil
}
