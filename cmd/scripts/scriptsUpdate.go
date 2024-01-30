package scripts

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var scriptsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a private script",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		name, id, err := updateScript(file)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}

		fmt.Printf("Succesfuly imported %s (%s)\n", name, id)
	},
}

func init() {
	ScriptsCmd.AddCommand(scriptsUpdateCmd)

	scriptsUpdateCmd.Flags().StringVar(&file, "file", "", "YAML file for script definition")
	scriptsUpdateCmd.MarkFlagRequired("file")
}

func updateScript(fileName string) (string, uuid.UUID, error) {
	return importScript(fileName, true)
}
