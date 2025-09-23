// Package commands_attachment — подкоманды для работы с файлами секретов.
package commands_attachment

import "github.com/spf13/cobra"

// NewAttachmentCmd - создает подкоманды для работы с файлами секретов.
func NewAttachmentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachment",
		Short: "Manage secret file(download, upload, update, list)",
	}

	cmd.AddCommand(NewDownloadAttachment())
	cmd.AddCommand(NewUploadAttachment())
	cmd.AddCommand(NewListAttachments())

	return cmd
}
