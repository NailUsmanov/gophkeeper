// Package commands — слой CLI-команд клиента GophKeeper.
//
// commands/auth отвечает за регистрацию, логирование и завершение сессии пользователя.
package commands_auth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// NewLoginCmd создаёт подкоманду "login".
func NewLoginCmd() *cobra.Command {

	var (
		email    string
		password string
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against server and save session",
		Long: `Performs POST /api/v1/login, reads auth_token cookie and persists it locally.

After a successful login, all subsequent commands will automatically include the token.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1) узнаём адрес сервера из корневого persistent-флага --server
			serverURL, err := cmd.Root().PersistentFlags().GetString("server")
			if err != nil {
				return err
			}
			if serverURL == "" {
				serverURL = "http://localhost:8080"
			}

			// 2) email
			email = strings.TrimSpace(email)
			if email == "" {
				return fmt.Errorf("email is required (use --email)")
			}

			// 3) пароль: если пуст — спросим интерактивно без эха
			if password == "" {
				fd := int(os.Stdin.Fd())

				if !term.IsTerminal(fd) {
					return fmt.Errorf("stdin is not a terminal; pass --password or use an interactive terminal")
				}
				fmt.Print("Password: ")
				// term.ReadPassword читает с tty без эха; по завершении возвращаем каретку
				b, err := term.ReadPassword(fd)
				fmt.Println()
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
				password = string(b)
			}

			// 4) создаём API-клиент
			cl, err := newClient(serverURL)
			if err != nil {
				return err
			}

			// 5) Контекст с таймаутом.
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()

			// 6) запрос к серверу
			user, token, err := cl.Login(ctx, email, password)
			if err != nil {
				return err
			}
			// 7) сохраняем токен локально
			if err := saveToken(token); err != nil {
				return fmt.Errorf("save session: %w", err)
			}

			// 8) Вывод команды.
			cmd.Printf("Logged in as %s (user id %s)\n", user.Email, user.ID)
			return nil
		},
	}

	// Локальные флаги этой команды:
	cmd.Flags().StringVarP(&email, "email", "e", "", "email address to authenticate")
	cmd.Flags().StringVarP(&password, "password", "p", "", "password (leave empty to be prompted)")

	return cmd
}
