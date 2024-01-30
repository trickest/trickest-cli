package scripts

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/cmd/library"
)

var jsonOutput bool

var scriptsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List private scripts",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		err := listScripts(jsonOutput)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	ScriptsCmd.AddCommand(scriptsListCmd)

	scriptsListCmd.Flags().BoolVar(&jsonOutput, "json", false, "Display output in JSON format")
}

func listScripts(jsonOutput bool) error {
	scripts, err := ListPrivateScripts("")
	if err != nil {
		return fmt.Errorf("couldn't list private scripts: %w", err)
	}

	if len(scripts) == 0 {
		return fmt.Errorf("couldn't find any private scripts. Did you mean `library list scripts`?")
	}

	library.PrintScripts(scripts, jsonOutput)
	return nil
}
