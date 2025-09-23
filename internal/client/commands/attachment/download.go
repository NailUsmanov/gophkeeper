// Package commands — слой CLI-команд клиента GophKeeper.
//
// commands/attachment осуществляет загрузку, скачивание, выдачу списка файлов секрета пользователя.
package commands_attachment

import (
	"context"
	"fmt"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/client/session"
	"github.com/NailUsmanov/gophkeeper/internal/client/transport"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewDownloadAttachment - используется для загрузки файла секрета.
func NewDownloadAttachment() *cobra.Command {
	var destPath string
	cmd := cobra.Command{
		Use:   "download <id>",
		Short: "Download attachment from server by ID",
		Args:  cobra.ExactArgs(1),
		Long:  "Performs GET /api/v1/attachments/{id} to get attachment for the user.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Получаем id
			id := args[0]
			if _, err := uuid.Parse(id); err != nil {
				return fmt.Errorf("id must be a valid UUID: %w", err)
			}

			// Сервер URL
			serverURL, err := cmd.Root().PersistentFlags().GetString("server")
			if err != nil {
				return err
			}
			if serverURL == "" {
				serverURL = "http://localhost:8080"
			}

			// берем токен
			token, err := session.LoadToken()
			if err != nil {
				return fmt.Errorf("load token: %w", err)
			}
			if token == "" {
				return fmt.Errorf("not logged in (empty token). Run 'gk login' first")
			}

			// Создаем клиента и ставим токен
			cl, err := transport.NewClient(serverURL)
			if err != nil {
				return err
			}
			cl.SetToken(token)

			// Контекст с таймаутом
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			// Запрос к серверу
			if err := cl.DownloadAttachment(ctx, id, destPath); err != nil {
				return fmt.Errorf("download attachment: %w", err)
			}
			fmt.Printf("Saved to %s\n", destPath)
			return nil

		},
	}
	cmd.Flags().StringVar(&destPath, "dest", "", "destination file path to save (e.g. ./demo.txt)")
	_ = cmd.MarkFlagRequired("dest")

	return &cmd
}
