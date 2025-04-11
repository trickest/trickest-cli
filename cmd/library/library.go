package library

import (
	"github.com/spf13/cobra"
)

type Config struct {
	Token   string
	BaseURL string

	JSONOutput bool
}

var cfg = &Config{}

// LibraryCmd represents the library command
var LibraryCmd = &cobra.Command{
	Use:   "library",
	Short: "Browse workflows and tools in the Trickest library",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	LibraryCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = command.Flags().MarkHidden("space")
		_ = command.Flags().MarkHidden("project")
		_ = command.Flags().MarkHidden("workflow")

		command.Root().HelpFunc()(command, strings)
	})
}
