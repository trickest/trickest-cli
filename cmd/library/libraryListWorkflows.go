package library

import (
	"encoding/json"
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
			printWorkflows(workflows, jsonOutput)
		} else {
			fmt.Println("Couldn't find any workflow in the library!")
		}
	},
}

func init() {
	libraryListCmd.AddCommand(libraryListWorkflowsCmd)
	libraryListWorkflowsCmd.Flags().BoolVar(&jsonOutput, "json", false, "Display output in JSON format")
}

func printWorkflows(workflows []types.Workflow, jsonOutput bool) {
	var output string

	if jsonOutput {
		data, err := json.Marshal(workflows)
		if err != nil {
			fmt.Println("Error marshalling project data")
			return
		}
		output = string(data)
	} else {
		tree := treeprint.New()
		tree.SetValue("Workflows")
		for _, workflow := range workflows {
			wfSubBranch := tree.AddBranch("\U0001f9be " + workflow.Name) //🦾
			if workflow.Description != "" {
				wfSubBranch.AddNode("\U0001f4cb \033[3m" + workflow.Description + "\033[0m") //📋
			}
		}

		output = tree.String()
	}
	fmt.Println(output)
}
