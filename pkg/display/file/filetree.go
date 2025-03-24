package display

import (
	"fmt"
	"io"

	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/xlab/treeprint"
)

const (
	fileEmoji  = "\U0001f4c4" // 📄
	sizeEmoji  = "\U0001f522" // 🔢
	dateEmoji  = "\U0001f4c5" // 📅
	dateFormat = "2006-01-02 15:04:05"
)

// PrintFiles writes the list of files in tree format to the given writer
func PrintFiles(w io.Writer, files []trickest.File) error {
	tree := treeprint.New()
	tree.SetValue("Files")
	for _, file := range files {
		fileSubBranch := tree.AddBranch(fileEmoji + " " + file.Name)
		fileSubBranch.AddNode(sizeEmoji + " " + file.PrettySize)
		fileSubBranch.AddNode(dateEmoji + " " + file.ModifiedDate.Format(dateFormat))
	}

	_, err := fmt.Fprintln(w, tree.String())
	return err
}
