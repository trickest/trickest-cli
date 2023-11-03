package library

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/cmd/list"
	"github.com/trickest/trickest-cli/types"
	"github.com/xlab/treeprint"
)

// libraryListToolsCmd represents the libraryListTools command
var libraryListToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "List tools from the Trickest library",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		tools := list.GetTools(math.MaxInt, "", "")
		if len(tools) > 0 {
			printTools(tools, jsonOutput)
		} else {
			fmt.Println("Couldn't find any tool in the library!")
		}
	},
}

func init() {
	libraryListCmd.AddCommand(libraryListToolsCmd)
	libraryListToolsCmd.Flags().BoolVar(&jsonOutput, "json", false, "Display output in JSON format")
}

func printTools(tools []types.Tool, jsonOutput bool) {
	var output string
	if jsonOutput {
		data, err := json.Marshal(tools)
		if err != nil {
			fmt.Println("Error marshalling project data")
			return
		}
		output = string(data)
	} else {
		tree := treeprint.New()
		tree.SetValue("Tools")
		for _, tool := range tools {
			branch := tree.AddBranch(tool.Name + " [" + strings.TrimPrefix(tool.SourceURL, "https://") + "]")
			branch.AddNode("\U0001f4cb \033[3m" + tool.Description + "\033[0m") //📋
		}

		output = tree.String()
	}

	fmt.Println(output)
}
