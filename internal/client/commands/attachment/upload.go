// Package commands — слой CLI-команд клиента GophKeeper.
//
// commands/attachment осуществляет загрузку, скачивание, выдачу списка файлов секрета пользователя.
package commands_attachment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/client/session"
	"github.com/NailUsmanov/gophkeeper/internal/client/transport"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewUploadAttachment - используется для выгрузки файла секрета в сеть.
func NewUploadAttachment() *cobra.Command {
	var secretID, filePath string
	cmd := cobra.Command{
		Use:   "upload",
		Short: "Upload attachment for a secret",
		Long:  "Uploads a local file and attaches it to the specified secret on the server.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// получаем id и валидируем
			if _, err := uuid.Parse(secretID); err != nil {
				return fmt.Errorf("--secret-id must be a valid UUID: %w", err)
			}

			// Проверяем существование файла.
			if filePath == "" {
				return fmt.Errorf("--file is required")
			}
			abs, err := filepath.Abs(filePath)
			if err != nil {
				return fmt.Errorf("abs path: %w", err)
			}
			fi, err := os.Stat(abs)
			if err != nil {
				return fmt.Errorf("stat file: %w", err)
			}
			if fi.IsDir() {
				return fmt.Errorf("file is a directory: %w", err)
			}

			// URL сервера
			serverURL, err := cmd.Root().PersistentFlags().GetString("server")
			if err != nil {
				return err
			}

			// Получаем токен
			token, err := session.LoadToken()
			if err != nil {
				return fmt.Errorf("not logged in: %w", err)
			}
			if token == "" {
				return fmt.Errorf("not logged in (empty token)")
			}

			// Создаем клиента
			cl, err := transport.NewClient(serverURL)
			if err != nil {
				return fmt.Errorf("new client: %w", err)
			}
			cl.SetToken(token)

			// Контекст с таймаутом.
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()

			// Запрос к серверу.
			meta, err := cl.UploadAttachment(ctx, secretID, filePath)
			if err != nil {
				return fmt.Errorf("upload attachment: %w", err)
			}

			// Вывод ответа.
			b, _ := json.MarshalIndent(meta, "", "   ")
			fmt.Println(string(b))
			return nil

		},
	}
	// Локальные флаги
	cmd.Flags().StringVar(&secretID, "secret-id", "", "UUID of the secret to attach the file to")
	cmd.Flags().StringVar(&filePath, "file", "", "path to a local file to upload")
	_ = cmd.MarkFlagRequired("secret-id")
	_ = cmd.MarkFlagRequired("file")

	return &cmd
}
