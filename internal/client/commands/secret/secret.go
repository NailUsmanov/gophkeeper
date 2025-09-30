// Package commands_secret — подкоманды для работы с секретами.
package commands_secret

import "github.com/spf13/cobra"

// NewSecretCmd создаёт родительскую команду "secret".
func NewSecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage secret (create, update, list, delete)",
	}

	// Добавляем подкоманды
	cmd.AddCommand(NewCreateSecret())
	cmd.AddCommand(NewGetSecret())
	cmd.AddCommand(NewListSecrets())
	cmd.AddCommand(NewUpdateSecret())

	return cmd
}
