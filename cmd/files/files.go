package files

import (
	"github.com/spf13/cobra"
)

var (
	FileNames string
)

// filesCmd represents the files command
var FilesCmd = &cobra.Command{
	Use:   "files",
	Short: "Manage files in the Trickest file storage",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	FilesCmd.PersistentFlags().StringVar(&FileNames, "file-name", "", "File name")
	FilesCmd.MarkPersistentFlagRequired("file-name")

	FilesCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = FilesCmd.Flags().MarkHidden("workflow")
		_ = FilesCmd.Flags().MarkHidden("project")
		_ = FilesCmd.Flags().MarkHidden("space")
		_ = FilesCmd.Flags().MarkHidden("url")

		command.Root().HelpFunc()(command, strings)
	})
}
