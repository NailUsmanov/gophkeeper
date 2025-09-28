// Package commands — слой CLI-команд клиента GophKeeper.
//
// commands/attachment осуществляет загрузку, скачивание, выдачу списка файлов секрета пользователя.
package commands_attachment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/client/session"
	"github.com/NailUsmanov/gophkeeper/internal/client/transport"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewListAttachments - реализует выполнение комадны выдачи списка файлов секрета.
func NewListAttachments() *cobra.Command {
	var flags struct {
		secretID string
		limit    int
		offset   int
	}

	cmd := cobra.Command{
		Use:   "list",
		Short: "Get list of attachments",
		Long:  "Performs GET /api/v1/attachments to get list of attachments of user.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runListAttachment(cmd, flags.secretID, flags.limit, flags.offset)
			return err
		},
	}
	cmd.Flags().StringVar(&flags.secretID, "secret-id", "", "id of the secret")
	_ = cmd.MarkFlagRequired("secret-id")
	cmd.Flags().IntVar(&flags.limit, "limit", 20, "limit of the secrets")
	cmd.Flags().IntVar(&flags.offset, "offset", 0, "offset of the secrets")
	return &cmd
}

func runListAttachment(cmd *cobra.Command, secretID string, limit, offset int) error {
	// Валидация секретID и лимит, оффсет
	if secretID != "" {
		if _, err := uuid.Parse(secretID); err != nil {
			return fmt.Errorf("--secret-id must be a valid UUID: %w", err)
		}
	}
	if limit < 0 {
		return fmt.Errorf("--limit must be >= 0")
	}
	if offset < 0 {
		return fmt.Errorf("--offset must be >= 0")
	}
	// Получаем URL сервера
	serverURL, err := cmd.Root().PersistentFlags().GetString("server")
	if err != nil {
		return err
	}
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// Берем токен
	token, err := session.LoadToken()
	if err != nil {
		return fmt.Errorf("load token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("not logged in (empty token). Run 'gk login' first")
	}

	// Создаем клиента
	cl, err := transport.NewClient(serverURL)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}
	cl.SetToken(token)

	// Контекст с таймаутом
	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	// Вызов АПИ
	items, total, err := cl.ListAttachments(ctx, secretID, limit, offset)
	if err != nil {
		return fmt.Errorf("list attachments: %w", err)
	}
	if total == 0 {
		fmt.Println("No attachments found")
		return nil
	}

	// Вывод ответа
	out := struct {
		Items []transport.AttachmentMeta `json:"items"`
		Total int                        `json:"total"`
	}{
		Items: items,
		Total: total,
	}
	b, _ := json.MarshalIndent(out, "", "   ")
	fmt.Println(string(b))

	return nil

}
