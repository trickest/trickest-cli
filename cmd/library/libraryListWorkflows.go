package library

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/cmd/list"
	"github.com/trickest/trickest-cli/types"
	"github.com/xlab/treeprint"
)

// libraryListWorkflowsCmd represents the libraryListWorkflows command
var libraryListWorkflowsCmd = &cobra.Command{
	Use:   "workflows",
	Short: "List workflows from the Trickest library",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		workflows := list.GetWorkflows(uuid.Nil, uuid.Nil, "", true)
		if len(workflows) > 0 {
			printWorkflows(workflows)
		} else {
			fmt.Println("Couldn't find any workflow in the library!")
		}
	},
}

func init() {
	libraryListCmd.AddCommand(libraryListWorkflowsCmd)
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
