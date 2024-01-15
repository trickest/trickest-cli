package tools

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/client/request"
)

var (
	toolID   string
	toolName string
)

var toolsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a private tool integration",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if toolName == "" && toolID == "" {
			cmd.Help()
			return
		}

		if toolName != "" {
			id, err := getToolIDByName(toolName)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			toolID = id.String()
		}

		err := deleteTool(toolID)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}

		fmt.Printf("Succesfuly deleted %s\n", toolID)
	},
}

func init() {
	ToolsCmd.AddCommand(toolsDeleteCmd)

	toolsDeleteCmd.Flags().StringVar(&toolID, "id", "", "ID of the tool to delete")
	toolsDeleteCmd.Flags().StringVar(&toolName, "name", "", "Name of the tool to delete")
}

func deleteTool(toolID string) error {
	resp := request.Trickest.Delete().DoF("library/tool/%s/", toolID)
	if resp == nil {
		return fmt.Errorf("couldn't delete %s: invalid response", toolID)
	}

	if resp.Status() == http.StatusNoContent {
		return nil
	} else {
		return fmt.Errorf("couldn't delete %s: unexpected status code (%d)", toolID, resp.Status())
	}
}
