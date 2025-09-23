// Package commands — слой CLI-команд клиента GophKeeper.
//
// commands/secret осуществляет создание, выдачу, удаление секрета пользователя.
package commands_secret

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

// NewUpdateSecret - команда для обновления секрета.
func NewUpdateSecret() *cobra.Command {
	var title, dataJSON string
	var version int

	cmd := cobra.Command{
		Use:   "update <id>",
		Short: "Update secret by ID",
		Long:  "Performs PUT /api/v1/secrets/{id} with optimistic locking by version.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// получаем id
			id := args[0]
			if _, err := uuid.Parse(id); err != nil {
				return fmt.Errorf("id must be a valid UUID: %w", err)
			}

			// Получаем URL сервера
			serverURL, err := cmd.Root().PersistentFlags().GetString("server")
			if err != nil {
				return err
			}
			if serverURL == "" {
				serverURL = "http://localhost:8080"
			}

			// берем токен из локальной сети.
			token, err := session.LoadToken()
			if err != nil {
				return fmt.Errorf("load token: %w", err)
			}
			if token == "" {
				return fmt.Errorf("not logged in (empty token)")
			}

			// проверяем версию
			if version <= 0 {
				return fmt.Errorf("--version must be > 0 (you must send the version you edited)")
			}

			// Парсим JSON с данными
			var data map[string]interface{}
			if dataJSON != "" {
				if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
					return fmt.Errorf("--data must be valid JSON: %w", err)
				}
			}

			// DTO для запроса
			req := transport.UpdateSecretRequest{
				Version: version,
			}
			if title != "" {
				req.Title = title
			}
			if data != nil {
				req.Data = data
			}

			// Создаем клиента
			cl, err := transport.NewClient(serverURL)
			if err != nil {
				return err
			}
			cl.SetToken(token)

			// Создаем контекст
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()

			// Запрос к серверу.
			updated, err := cl.UpdateSecret(ctx, id, req)
			if err != nil {
				return fmt.Errorf("update secret: %w", err)
			}

			// Вывод команды
			b, _ := json.MarshalIndent(updated, "", "    ")
			fmt.Println(string(b))
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "new title (optional)")
	cmd.Flags().StringVar(&dataJSON, "data", "", "new JSON data (optional), e.g. '{\"note\":\"updated\"}'")
	cmd.Flags().IntVar(&version, "version", 0, "current version of the secret (required)")

	_ = cmd.MarkFlagRequired("version")

	return &cmd
}
