package handlers

import (
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
)

func Test_toAttachmentResponse(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		name string
		in   models.AttachmentMeta
	}{
		{
			name: "basic",
			in: models.AttachmentMeta{
				ID:          "att-1",
				SecretID:    "sec-1",
				FileName:    "file.txt",
				ContentType: "text/plain",
				Size:        123,
				CreatedAt:   now,
			},
		},
		{
			name: "zero time formatted empty",
			in: models.AttachmentMeta{
				ID: "att-2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toAttachmentResponse(tt.in)
			if got.ID != tt.in.ID || got.SecretID != tt.in.SecretID || got.FileName != tt.in.FileName ||
				got.ContentType != tt.in.ContentType || got.Size != tt.in.Size {
				t.Fatalf("fields mismatch: got=%+v want from in=%+v", got, tt.in)
			}
			// created_at проверим на пустоту/непустоту
			if tt.in.CreatedAt.IsZero() && got.CreatedAt != "" {
				t.Fatalf("CreatedAt expected empty, got %q", got.CreatedAt)
			}
			if !tt.in.CreatedAt.IsZero() && got.CreatedAt == "" {
				t.Fatalf("CreatedAt expected non-empty, got empty")
			}
		})
	}
}

func Test_toAttachmentList(t *testing.T) {
	items := []models.AttachmentMeta{
		{ID: "1"},
		{ID: "2"},
	}
	resp := toAttachmentList(items, 10, 20, 0)
	if len(resp.Items) != 2 || resp.Total != 10 || resp.Limit != 20 || resp.Offset != 0 {
		t.Fatalf("unexpected list resp: %+v", resp)
	}
	if resp.Items[0].ID != "1" || resp.Items[1].ID != "2" {
		t.Fatalf("unexpected item IDs: %+v", resp.Items)
	}
}
