package tools

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var toolsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a private tool integration",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		name, id, err := updateTool(file)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}

		fmt.Printf("Succesfuly imported %s (%s)\n", name, id)
	},
}

func init() {
	ToolsCmd.AddCommand(toolsUpdateCmd)

	toolsUpdateCmd.Flags().StringVar(&file, "file", "", "YAML file for tool definition")
	toolsUpdateCmd.MarkFlagRequired("file")
}

func updateTool(fileName string) (string, uuid.UUID, error) {
	return importTool(fileName, true)
}
