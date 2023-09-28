package library

import (
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/cmd/list"
)

// librarySearchCmd represents the librarySearch command
var librarySearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for workflows and tools in the Trickest library",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		search := ""
		if len(args) > 0 {
			search = args[0]
		}
		tools := list.GetTools(math.MaxInt, search, "")
		workflows := list.GetWorkflows(uuid.Nil, uuid.Nil, search, true)
		if len(tools) > 0 {
			printTools(tools)
		} else {
			fmt.Println("Couldn't find any tool in the library that matches the search!")
		}
		if len(workflows) > 0 {
			printWorkflows(workflows)
		} else {
			fmt.Println("Couldn't find any workflow in the library that matches the search!")
		}
	},
}

func init() {
	LibraryCmd.AddCommand(librarySearchCmd)
}
