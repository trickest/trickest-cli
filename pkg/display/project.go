package display

import (
	"fmt"
	"io"

	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/xlab/treeprint"
)

// PrintProject writes the project details in tree format to the given writer
func PrintProject(w io.Writer, project trickest.Project) error {
	tree := treeprint.New()
	tree.SetValue(projectEmoji + "  " + project.Name)
	if project.Description != "" {
		tree.AddNode(descriptionEmoji + " \033[3m" + project.Description + "\033[0m")
	}
	if project.Workflows != nil && len(project.Workflows) > 0 {
		wfBranch := tree.AddBranch("Workflows")
		for _, workflow := range project.Workflows {
			wfSubBranch := wfBranch.AddBranch(workflowEmoji + " " + workflow.Name)
			if workflow.Description != "" {
				wfSubBranch.AddNode(descriptionEmoji + " \033[3m" + workflow.Description + "\033[0m")
			}
		}
	}

	_, err := fmt.Fprintln(w, tree.String())
	return err
}
