package store

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/xlab/treeprint"
	"math"
	"strings"
	"trickest-cli/cmd/list"
	"trickest-cli/types"
)

// storeListToolsCmd represents the storeListTools command
var storeListToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "List tools from the Trickest store",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		tools := list.GetTools(math.MaxInt, "", "")
		if len(tools) > 0 {
			printTools(tools)
		} else {
			fmt.Println("Couldn't find any tool in the store!")
		}
	},
}

func init() {
	storeListCmd.AddCommand(storeListToolsCmd)
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
