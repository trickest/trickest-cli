package library

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/types"
	"github.com/xlab/treeprint"
)

var (
	jsonOutput bool
)

// LibraryCmd represents the library command
var LibraryCmd = &cobra.Command{
	Use:   "library",
	Short: "Browse workflows and tools in the Trickest library",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	LibraryCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = command.Flags().MarkHidden("space")
		_ = command.Flags().MarkHidden("project")
		_ = command.Flags().MarkHidden("workflow")

		command.Root().HelpFunc()(command, strings)
	})
}

func PrintTools(tools []types.Tool, jsonOutput bool) {
	var output string
	if jsonOutput {
		data, err := json.Marshal(tools)
		if err != nil {
			fmt.Println("Error marshalling project data")
			return
		}
		output = string(data)
	} else {
		tree := treeprint.New()
		tree.SetValue("Tools")
		for _, tool := range tools {
			branch := tree.AddBranch(tool.Name + " [" + strings.TrimPrefix(tool.SourceURL, "https://") + "]")
			branch.AddNode("\U0001f4cb \033[3m" + tool.Description + "\033[0m") //📋
		}

		output = tree.String()
	}

	fmt.Println(output)
}

func PrintScripts(scripts []types.Script, jsonOutput bool) {
	var output string
	if jsonOutput {
		data, err := json.Marshal(scripts)
		if err != nil {
			fmt.Println("Error marshalling project data")
			return
		}
		output = string(data)
	} else {
		tree := treeprint.New()
		tree.SetValue("Scripts")
		for _, script := range scripts {
			branch := tree.AddBranch(script.Name)
			branch.AddNode(script.Script.Source)
		}

		output = tree.String()
	}

	fmt.Println(output)
}
