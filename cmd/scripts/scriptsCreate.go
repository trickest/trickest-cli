package scripts

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var file string

var scriptsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new private script",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		name, id, err := createScript(file)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}

		fmt.Printf("Succesfuly imported %s (%s)\n", name, id)
	},
}

func init() {
	ScriptsCmd.AddCommand(scriptsCreateCmd)

	scriptsCreateCmd.Flags().StringVar(&file, "file", "", "YAML file for script definition")
	scriptsCreateCmd.MarkFlagRequired("file")
}

func createScript(fileName string) (string, uuid.UUID, error) {
	return importScript(fileName, false)
}
