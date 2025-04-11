package display

import (
	"fmt"
	"io"
	"strings"

	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/xlab/treeprint"
)

// PrintTools writes the tools list in tree format to the given writer
func PrintTools(w io.Writer, tools []trickest.Tool) error {
	tree := treeprint.New()
	tree.SetValue("Tools")
	for _, tool := range tools {
		branch := tree.AddBranch(tool.Name + " [" + strings.TrimPrefix(tool.SourceURL, "https://") + "]")
		branch.AddNode(descriptionEmoji + " \033[3m" + tool.Description + "\033[0m")
	}

	_, err := fmt.Fprintln(w, tree.String())
	return err
}
