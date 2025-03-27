package display

import (
	"fmt"
	"io"

	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/xlab/treeprint"
)

// PrintWorkflow writes the workflow details in tree format to the given writer
func PrintWorkflow(w io.Writer, workflow trickest.Workflow) error {
	tree := treeprint.New()
	tree.SetValue(workflowEmoji + " " + workflow.Name)
	if workflow.Description != "" {
		tree.AddNode(descriptionEmoji + " \033[3m" + workflow.Description + "\033[0m")
	}
	tree.AddNode("Author: " + workflow.Author)

	_, err := fmt.Fprintln(w, tree.String())
	return err
}

// PrintWorkflows writes the workflows list in tree format to the given writer
func PrintWorkflows(w io.Writer, workflows []trickest.Workflow) error {
	tree := treeprint.New()
	tree.SetValue("Workflows")
	for _, workflow := range workflows {
		wfSubBranch := tree.AddBranch(workflowEmoji + " " + workflow.Name)
		if workflow.Description != "" {
			wfSubBranch.AddNode(descriptionEmoji + " \033[3m" + workflow.Description + "\033[0m")
		}
	}

	_, err := fmt.Fprintln(w, tree.String())
	return err
}
