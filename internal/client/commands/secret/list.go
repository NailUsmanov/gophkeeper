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
	"github.com/spf13/cobra"
)

// NewListSecrets - используется для получения секретов пользователя с пагинацией.
func NewListSecrets() *cobra.Command {
	var limit, offset int
	var typ string
	var updatedAfterStr string

	cmd := cobra.Command{
		Use:   "list",
		Short: "List secrets of current user",
		Long:  "Performs GET /api/v1/secrets and shows secrets of the logged-in user.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// узнаем адрес сервера из корневого persistent-флага --server
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

			// Проверим тип
			switch typ {
			case "", "note", "password", "card", "file":
			default:
				return fmt.Errorf("--type must be one of: note|password|card|file")
			}
			// Создаем клиента
			cl, err := transport.NewClient(serverURL)
			if err != nil {
				return err
			}
			cl.SetToken(token)

			// updated-after
			var updatedAfter time.Time
			if updatedAfterStr != "" {
				updatedAfter, err = time.Parse(time.RFC3339, updatedAfterStr)
				if err != nil {
					return fmt.Errorf("invalid --updated-after: must be RFC3339, got %s", updatedAfterStr)
				}
			}

			// Создаем контекст
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()

			// Запрос к серверу.
			secrets, total, err := cl.ListSecrets(ctx, limit, offset, typ, updatedAfter)
			if err != nil {
				return fmt.Errorf("list secrets: %w", err)
			}
			if total == 0 {
				fmt.Println("No secrets found")
				return nil
			}
			out := struct {
				Items []transport.SecretResponse `json:"items"`
				Total int                        `json:"total"`
			}{
				Items: secrets,
				Total: total,
			}
			b, _ := json.MarshalIndent(out, " ", "   ")
			fmt.Println(string(b))
			return nil

		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "limit of the secrets")
	cmd.Flags().IntVar(&offset, "offset", 0, "offset of the secrets")
	cmd.Flags().StringVar(&typ, "type", "", "type of the secrets")
	cmd.Flags().StringVar(&updatedAfterStr, "updated-after", "", "filter by updated after (RFC3339)")
	return &cmd
}
