package scripts

import (
	"github.com/spf13/cobra"
)

var ScriptsCmd = &cobra.Command{
	Use:   "scripts",
	Short: "Manage private scripts",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	ScriptsCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = ScriptsCmd.Flags().MarkHidden("workflow")
		_ = ScriptsCmd.Flags().MarkHidden("project")
		_ = ScriptsCmd.Flags().MarkHidden("space")
		_ = ScriptsCmd.Flags().MarkHidden("url")

		command.Root().HelpFunc()(command, strings)
	})
}
