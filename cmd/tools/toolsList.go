package tools

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/cmd/library"
)

var jsonOutput bool

var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List private tool integrations",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		err := listTools(jsonOutput)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	ToolsCmd.AddCommand(toolsListCmd)

	toolsListCmd.Flags().BoolVar(&jsonOutput, "json", false, "Display output in JSON format")
}

func listTools(jsonOutput bool) error {
	tools, err := ListPrivateTools("")
	if err != nil {
		return fmt.Errorf("couldn't list private tools: %w", err)
	}

	if len(tools) == 0 {
		return fmt.Errorf("couldn't find any private tools. Did you mean `library list tools`?")
	}

	library.PrintTools(tools, jsonOutput)
	return nil
}
