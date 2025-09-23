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

// NewCreateSecret - создает команду для сборки секрета.
func NewCreateSecret() *cobra.Command {
	// пользователь передает тип, заголовок и данные секрета через флаги.
	var (
		typ, title, data string
	)
	cmd := cobra.Command{
		Use:   "create",
		Short: "Creating secret on the server",
		Long:  "Performs POST /api/v1/secrets to create a new secret for the user.",
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

			// создаем API клиента
			cl, err := transport.NewClient(serverURL)
			if err != nil {
				return err
			}
			cl.SetToken(token)

			// парсим JSON из --data
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				return fmt.Errorf("--data must be valid JSON object: %w", err)
			}

			// DTO
			req := transport.CreateSecretRequest{
				Type:  typ,
				Title: title,
				Data:  payload, // если пользователь передал JSON строкой
			}

			// Контекст с таймаутом.
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()

			// Запрос к серверу.
			secret, err := cl.CreateSecret(ctx, req)
			if err != nil {
				return err
			}

			// Вывод команды.
			b, _ := json.MarshalIndent(secret, "", "   ")
			fmt.Println(string(b))
			return nil

		},
	}

	// локальные флаги
	cmd.Flags().StringVar(&typ, "type", "text", "type of secret(password|note|card|file)")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		switch typ {
		case "password", "note", "card", "file":
			return nil
		default:
			return fmt.Errorf("--type must be one of: password|note|card|file")
		}
	}
	cmd.Flags().StringVar(&title, "title", "", "title of the secret")
	_ = cmd.MarkFlagRequired("title")
	cmd.Flags().StringVar(&data, "data", "{}", "JSON data of the secret")
	return &cmd
}
