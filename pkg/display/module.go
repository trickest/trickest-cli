package display

import (
	"fmt"
	"io"

	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/xlab/treeprint"
)

// PrintModules writes the modules list in tree format to the given writer
func PrintModules(w io.Writer, modules []trickest.Module) error {
	tree := treeprint.New()
	tree.SetValue("Modules")
	for _, module := range modules {
		mdSubBranch := tree.AddBranch(moduleEmoji + " " + module.Name)
		if module.Description != "" {
			mdSubBranch.AddNode(descriptionEmoji + " \033[3m" + module.Description + "\033[0m")
		}
		inputSubBranch := mdSubBranch.AddBranch(inputEmoji + " Inputs")
		for _, input := range module.Data.Inputs {
			inputSubBranch.AddNode(input.Name)
		}
		outputSubBranch := mdSubBranch.AddBranch(outputEmoji + " Outputs")
		for _, output := range module.Data.Outputs {
			outputSubBranch.AddNode(*output.ParameterName)
		}
	}

	_, err := fmt.Fprintln(w, tree.String())
	return err
}
