package library

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/cmd/list"
	"github.com/trickest/trickest-cli/util"
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
		workflows := util.GetWorkflows(uuid.Nil, uuid.Nil, search, true)
		if jsonOutput {
			results := map[string]interface{}{
				"tools":     tools,
				"workflows": workflows,
			}
			data, err := json.Marshal(results)
			if err != nil {
				fmt.Println("Error marshalling project data")
				return
			}
			output := string(data)
			fmt.Println(output)
		} else {
			if len(tools) > 0 {
				PrintTools(tools, jsonOutput)
			} else {
				fmt.Println("Couldn't find any tool in the library that matches the search!")
			}
			if len(workflows) > 0 {
				printWorkflows(workflows, jsonOutput)
			} else {
				fmt.Println("Couldn't find any workflow in the library that matches the search!")
			}
		}
	},
}

func init() {
	LibraryCmd.AddCommand(librarySearchCmd)
	librarySearchCmd.Flags().BoolVar(&jsonOutput, "json", false, "Display output in JSON format")
}
