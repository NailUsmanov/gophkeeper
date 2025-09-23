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

// rootCmd — ОДНА корневая команда. Всё остальное — подкоманды (login, secret, attachment...).
// Поле Use — что пишем для вызова команды. Short будет выдан при --help.
// long - многострочное описание.
var rootCmd = &cobra.Command{
	Use:   "gk",
	Short: "GophKeeper CLI Client",
	Long: `GophKeeper CLI — клиент для регистрации, логина и работы с секретами/вложениями.
Примеры:
  gk version
  gk login --email user@example.com --password secret
  gk secret list --type note
  gk attachment upload --secret-id <id> --file ./path/to/file
`,
	// SilenceUsage: true — не печать usage на каждую ошибку (иначе шумно).
	SilenceUsage: true,
	// SilenceErrors: true — отдаём ошибку наверх, main печатает ошибки через log.Fatal.
	SilenceErrors: true,
	// PersistentPreRunE — действие, которое выполнится перед ЛЮБОЙ подкомандой.
	// Используется для инициализации чтения конфига, валидации флагов.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if serverURL == "" {
			if v := os.Getenv("GK_SERVER_URL"); v != "" {
				serverURL = v
			}
		}
		if serverURL == "" {
			return fmt.Errorf("server URL is required (flag --server or env GL_SERVER_URL)")
		}
		return nil
	},
}

// Execute — вход из main.go.
// Сохраняет build-инфо, навешивает глобальные флаги, регистрирует подкоманды.
// Вызывает rootCmd.Execute() для парсинга аргументов и запуск нужной подкоманды.
func Execute(version, date, commit string) error {
	buildVersion, buildDate, buildCommit = version, date, commit

	// Глобальные (persistent) флаги — видны всем подкомандам: gk <sub> --server http://...
	rootCmd.PersistentFlags().StringVar(
		&serverURL, "server", "http://localhost:8080",
		"base URL of the GophKeeper server (can be GK_SERVER_URL)",
	)
	// rootCmd.AddCommand(newSecretCmd())      // у которой будут подкоманды list/get/create/update
	// rootCmd.AddCommand(newAttachmentCmd())  // у которой будут upload/download/list
	// команда проверяет версию:
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(auth.NewLoginCmd())
	rootCmd.AddCommand(auth.NewRegisterCmd())
	rootCmd.AddCommand(auth.NewLogoutCmd())

	// добавляем группу secret
	rootCmd.AddCommand(secret.NewSecretCmd())

	// добавляем группу attachment
	rootCmd.AddCommand(attachment.NewAttachmentCmd())
	return rootCmd.Execute()
}

// Command "gk version"
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print client build information",
		Long:  "Показывает версию, дату, коммит сборки конкретного CLI-клиентаю.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Version: %s\nBuild date: %s\nCommit: %s\n", buildVersion, buildDate, buildCommit)
			return nil
		},
	}
}
