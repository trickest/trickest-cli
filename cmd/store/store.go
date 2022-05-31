package store

import (
	"github.com/spf13/cobra"
)

// StoreCmd represents the store command
var StoreCmd = &cobra.Command{
	Use:   "store",
	Short: "Browse workflows and tools in the Trickest store",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	StoreCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = command.Flags().MarkHidden("space")
		_ = command.Flags().MarkHidden("project")
		_ = command.Flags().MarkHidden("workflow")

		command.Root().HelpFunc()(command, strings)
	})
}
