package tools

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var file string

var toolsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new private tool integration",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		name, id, err := createTool(file)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}

		fmt.Printf("Succesfuly imported %s (%s)\n", name, id)
	},
}

func init() {
	ToolsCmd.AddCommand(toolsCreateCmd)

	toolsCreateCmd.Flags().StringVar(&file, "file", "", "YAML file for tool definition")
	toolsCreateCmd.MarkFlagRequired("file")
}

func createTool(fileName string) (string, uuid.UUID, error) {
	return importTool(fileName, false)
}
