package library

import (
	"github.com/spf13/cobra"
)

// libraryListCmd represents the libraryList command
var libraryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List modules,workflows, and tools from the Trickest library",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	LibraryCmd.AddCommand(libraryListCmd)
}
