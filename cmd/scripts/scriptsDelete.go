package scripts

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/client/request"
)

var (
	scriptID   string
	scriptName string
)

var scriptsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a private script",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if scriptName == "" && scriptID == "" {
			cmd.Help()
			return
		}

		if scriptName != "" {
			id, err := getScriptIDByName(scriptName)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			scriptID = id.String()
		}

		err := deleteScript(scriptID)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}

		fmt.Printf("Succesfuly deleted %s\n", scriptID)
	},
}

func init() {
	ScriptsCmd.AddCommand(scriptsDeleteCmd)

	scriptsDeleteCmd.Flags().StringVar(&scriptID, "id", "", "ID of the script to delete")
	scriptsDeleteCmd.Flags().StringVar(&scriptName, "name", "", "Name of the script to delete")
}

func deleteScript(scriptID string) error {
	resp := request.Trickest.Delete().DoF("script/%s/", scriptID)
	if resp == nil {
		return fmt.Errorf("couldn't delete %s: invalid response", scriptID)
	}

	if resp.Status() == http.StatusNoContent {
		return nil
	} else {
		return fmt.Errorf("couldn't delete %s: unexpected status code (%d)", scriptID, resp.Status())
	}
}
