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

// storeListCmd represents the storeList command
var storeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workflows from the Trickest store",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		tools := list.GetTools(math.MaxInt, "", "")
		workflows := list.GetWorkflows("", true, "")
		if tools != nil && len(tools) > 0 {
			printTools(tools)
		} else {
			fmt.Println("Couldn't find any tool in the store!")
		}
		if workflows != nil && len(workflows) > 0 {
			printWorkflows(workflows)
		} else {
			fmt.Println("Couldn't find any workflow in the store!")
		}
	},
}

func init() {
	StoreCmd.AddCommand(storeListCmd)
}

func printWorkflows(workflows []types.WorkflowListResponse) {
	tree := treeprint.New()
	tree.SetValue("Workflows")
	for _, workflow := range workflows {
		wfSubBranch := tree.AddBranch("\U0001f9be " + workflow.Name) //🦾
		if workflow.Description != "" {
			wfSubBranch.AddNode("\U0001f4cb \033[3m" + workflow.Description + "\033[0m") //📋
		}
	}

	fmt.Println(tree.String())
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
