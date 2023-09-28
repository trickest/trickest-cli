package library

import (
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
			printTools(tools)
		} else {
			fmt.Println("Couldn't find any tool in the library!")
		}
	},
}

func init() {
	libraryListCmd.AddCommand(libraryListToolsCmd)
}

func printTools(tools []types.Tool) {
	tree := treeprint.New()
	tree.SetValue("Tools")
	for _, tool := range tools {
		branch := tree.AddBranch(tool.Name + " [" + strings.TrimPrefix(tool.SourceURL, "https://") + "]")
		branch.AddNode("\U0001f4cb \033[3m" + tool.Description + "\033[0m") //📋
	}

	fmt.Println(tree.String())
}
