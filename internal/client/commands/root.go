// Package commands — слой CLI-команд клиента GophKeeper.
// Здесь описываются корневая команда (root) и все подкоманды.
// Основная задача пакета — распарсить аргументы/флаги и вызвать нужный сценарий.
package commands

import (
	"fmt"
	"os"

	attachment "github.com/NailUsmanov/gophkeeper/internal/client/commands/attachment"
	auth "github.com/NailUsmanov/gophkeeper/internal/client/commands/auth"
	secret "github.com/NailUsmanov/gophkeeper/internal/client/commands/secret"

	"github.com/spf13/cobra"
)

var (
	// значения версии, прокинутые из main.go
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"

	// общий флаг для всех подкоманд
	serverURL string
)

// NewRootCmd — конструктор корневой команды.
// Создаёт НОВЫЙ *cobra.Command на каждый вызов (без глобального singletons),
// чтобы тесты могли вызывать NewRootCmd многократно без "flag redefined".
func NewRootCmd(version, date, commit string) *cobra.Command {
	buildVersion, buildDate, buildCommit = version, date, commit

	cmd := &cobra.Command{
		Use:   "gk",
		Short: "GophKeeper CLI Client",
		Long: `GophKeeper CLI — клиент для регистрации, логина и работы с секретами/вложениями.
Примеры:
  gk version
  gk login --email user@example.com --password secret
  gk secret list --type note
  gk attachment upload --secret-id <id> --file ./path/to/file
`,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Проверка serverURL перед запуском любых подкоманд,
		// но оставляем дефолт, так что это не будет падать в тестах.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if serverURL == "" {
				if v := os.Getenv("GK_SERVER_URL"); v != "" {
					serverURL = v
				}
			}
			if serverURL == "" {
				return fmt.Errorf("server URL is required (flag --server or env GK_SERVER_URL)")
			}
			return nil
		},
	}

	// Глобальный флаг (persistent) — виден всем подкомандам
	cmd.PersistentFlags().StringVar(
		&serverURL, "server", "http://localhost:8080",
		"base URL of the GophKeeper server (can be GK_SERVER_URL)",
	)

	// Подкоманды
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(auth.NewLoginCmd())
	cmd.AddCommand(auth.NewRegisterCmd())
	cmd.AddCommand(auth.NewLogoutCmd())
	cmd.AddCommand(secret.NewSecretCmd())
	cmd.AddCommand(attachment.NewAttachmentCmd())

	return cmd
}

// Execute — точка входа из main.go: строит команду и запускает её.
func Execute(version, date, commit string) error {
	return NewRootCmd(version, date, commit).Execute()
}

// Command "gk version"
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print client build information",
		Long:  "Показывает версию, дату и коммит сборки CLI-клиента.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Version: %s\nBuild date: %s\nCommit: %s\n", buildVersion, buildDate, buildCommit)
			return nil
		},
	}
}
