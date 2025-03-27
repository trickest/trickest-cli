package tools

import (
	"github.com/spf13/cobra"
)

func init() {
	ToolsCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = ToolsCmd.Flags().MarkHidden("workflow")
		_ = ToolsCmd.Flags().MarkHidden("project")
		_ = ToolsCmd.Flags().MarkHidden("space")
		_ = ToolsCmd.Flags().MarkHidden("url")

		command.Root().HelpFunc()(command, strings)
	})
}

var ToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Manage private tools",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}
