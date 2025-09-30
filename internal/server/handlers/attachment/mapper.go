// Package attachment содержит HTTP-хендлеры и вспомогательные функции для вложений.
//
// mapper.go — преобразование доменных моделей (AttachmentMeta) в сетевые DTO.

package handlers

import (
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	thttp "github.com/NailUsmanov/gophkeeper/internal/server/transport/http"
)

// toRFC3339OrEmpty форматирует t как RFC3339 или возвращает пустую строку, если t.IsZero().
func toRFC3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// toAttachmentResponse преобразует доменную модель в DTO-ответ.
func toAttachmentResponse(m models.AttachmentMeta) thttp.AttachmentResponse {
	return thttp.AttachmentResponse{
		ID:          m.ID,
		SecretID:    m.SecretID,
		FileName:    m.FileName,
		ContentType: m.ContentType,
		Size:        m.Size,
		CreatedAt:   toRFC3339OrEmpty(m.CreatedAt),
	}
}

// toAttachmentList преобразует список AttachmentMeta в DTO-ответ AttachmentList.
func toAttachmentList(items []models.AttachmentMeta, total, limit, offset int) thttp.ListAttachmentResponse {
	out := make([]thttp.AttachmentResponse, 0, len(items))
	for _, it := range items {
		out = append(out, toAttachmentResponse(it))
	}
	return thttp.ListAttachmentResponse{
		Items:  out,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
}
