package store

import (
	"fmt"
	"github.com/spf13/cobra"
	"trickest-cli/cmd/list"
)

// storeSearchCmd represents the storeSearch command
var storeSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for workflows in the Trickest store",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		search := ""
		if len(args) > 0 {
			search = args[0]
		}
		workflows := list.GetWorkflows("", true, search)
		if workflows != nil && len(workflows) > 0 {
			printWorkflows(workflows)
		} else {
			fmt.Println("Couldn't find any workflow in the store that matches the search!")
		}
	},
}

func init() {
	StoreCmd.AddCommand(storeSearchCmd)

}
