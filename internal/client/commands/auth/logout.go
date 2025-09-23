// Package commands — слой CLI-команд клиента GophKeeper.
//
// commands/auth отвечает за регистрацию, логирование и завершение сессии пользователя.
package commands_auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewLogoutCmd - создает команду выхода из сессии.
func NewLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout and remove local session",
		Long: `Performs POST /api/v1/logout on the server (if token is valid)
and then deletes the saved session token locally.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Получаем адрес сервера из root-флага.
			serverURL, err := cmd.Root().PersistentFlags().GetString("server")
			if err != nil {
				return err
			}
			if serverURL == "" {
				serverURL = "http://localhost:8080"
			}

			// 2. Пробуем загрузить токен из локальной сети.
			token, err := loadToken()
			if err != nil || strings.TrimSpace(token) == "" {
				return fmt.Errorf("no session found (already logged out?)")
			}

			// 3. Делаем клиент.
			cl, err := newClient(serverURL)
			if err != nil {
				return err
			}

			// 4. Контекст с таймаутом.
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			// 5. Отправляем logout-запрос
			if err := cl.Logout(ctx, token); err != nil {
				fmt.Println("warning: logout request failed:", err)
			}

			// 6. Удаляем токен локально
			if err := clearSession(); err != nil {
				return fmt.Errorf("clear session: %w", err)
			}

			cmd.Println("Logged out successfully.")
			return nil
		},
	}
	return cmd
}
