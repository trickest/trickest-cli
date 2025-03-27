package display

import (
	"fmt"
	"io"

	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/xlab/treeprint"
)

// PrintSpace writes the space details in tree format to the given writer
func PrintSpace(w io.Writer, space trickest.Space) error {
	tree := treeprint.New()
	tree.SetValue(spaceEmoji + " " + space.Name)
	if space.Description != "" {
		tree.AddNode(descriptionEmoji + " \033[3m" + space.Description + "\033[0m")
	}
	if space.Projects != nil && len(space.Projects) > 0 {
		projectsBranch := tree.AddBranch("Projects")
		for _, project := range space.Projects {
			projectSubBranch := projectsBranch.AddBranch(projectEmoji + "  " + project.Name)
			if project.Description != "" {
				projectSubBranch.AddNode(descriptionEmoji + " \033[3m" + project.Description + "\033[0m")
			}
		}
	}
	if space.Workflows != nil && len(space.Workflows) > 0 {
		workflowsBranch := tree.AddBranch("Workflows")
		for _, workflow := range space.Workflows {
			workflowSubBranch := workflowsBranch.AddBranch(workflowEmoji + " " + workflow.Name)
			if workflow.Description != "" {
				workflowSubBranch.AddNode(descriptionEmoji + " \033[3m" + workflow.Description + "\033[0m")
			}
		}
	}

	_, err := fmt.Fprintln(w, tree.String())
	return err
}

// PrintSpaces writes the list of spaces in tree format to the given writer
func PrintSpaces(w io.Writer, spaces []trickest.Space) error {
	tree := treeprint.New()
	tree.SetValue("Spaces")
	for _, space := range spaces {
		branch := tree.AddBranch(spaceEmoji + " " + space.Name)
		if space.Description != "" {
			branch.AddNode(descriptionEmoji + " \033[3m" + space.Description + "\033[0m")
		}
	}

	_, err := fmt.Fprintln(w, tree.String())
	return err
}
