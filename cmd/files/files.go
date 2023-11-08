package files

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"
)

var (
	FileNames string
)

// filesCmd represents the files command
var FilesCmd = &cobra.Command{
	Use:   "files",
	Short: "Manage files in the Trickest file storage",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	FilesCmd.PersistentFlags().StringVar(&FileNames, "file-name", "", "File name or names (comma-separated)")
	FilesCmd.MarkPersistentFlagRequired("file-name")

	FilesCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = FilesCmd.Flags().MarkHidden("workflow")
		_ = FilesCmd.Flags().MarkHidden("project")
		_ = FilesCmd.Flags().MarkHidden("space")
		_ = FilesCmd.Flags().MarkHidden("url")

		command.Root().HelpFunc()(command, strings)
	})
}

func getMetadata(searchQuery string) ([]types.File, error) {
	resp := request.Trickest.Get().DoF("file/?search=%s&vault=%s", searchQuery, util.GetVault())
	if resp == nil || resp.Status() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status code: %d", resp.Status())
	}
	var metadata types.Files

	err := json.Unmarshal(resp.Body(), &metadata)
	if err != nil {
		return nil, fmt.Errorf("couldn't unmarshal file IDs response: %s", err)
	}

	return metadata.Results, nil
}
