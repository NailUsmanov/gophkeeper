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

// NewGetSecret - команда для получения секрета по ID.
func NewGetSecret() *cobra.Command {
	cmd := cobra.Command{
		Use:   "get <id>",
		Short: "Get secret from the server by ID",
		Args:  cobra.ExactArgs(1),
		Long:  "Performs GET /api/v1/secrets/{id} to get a secret for the user.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// получаем id
			id := args[0]
			if _, err := uuid.Parse(id); err != nil {
				return fmt.Errorf("id must be a valid UUID: %w", err)
			}
			// узнаем адрес сервера из корневого persistent-флага --server
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

			// создаем API клиента и ставим токен
			cl, err := transport.NewClient(serverURL)
			if err != nil {
				return err
			}
			cl.SetToken(token)

			// Контекст с таймаутом.
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()

			// Запрос к серверу.
			secret, err := cl.GetSecret(ctx, id)
			if err != nil {
				return fmt.Errorf("get secret: %w", err)
			}

			// Вывод команды.
			b, _ := json.MarshalIndent(secret, "", "   ")
			fmt.Println(string(b))
			return nil

		},
	}
	return &cmd
}
