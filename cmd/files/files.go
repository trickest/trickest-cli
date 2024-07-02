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
	Files string
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
	FilesCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = FilesCmd.Flags().MarkHidden("workflow")
		_ = FilesCmd.Flags().MarkHidden("project")
		_ = FilesCmd.Flags().MarkHidden("space")
		_ = FilesCmd.Flags().MarkHidden("url")

		command.Root().HelpFunc()(command, strings)
	})
}

func getMetadata(searchQuery string) ([]types.File, error) {
	pageSize := 100

	page := 1
	var allFiles []types.File
	for {
		resp := request.Trickest.Get().DoF("file/?search=%s&vault=%s&page_size=%d&page=%d", searchQuery, util.GetVault(), pageSize, page)
		if resp == nil || resp.Status() != http.StatusOK {
			return nil, fmt.Errorf("unexpected response status code: %d", resp.Status())
		}

		var metadata types.Files
		err := json.Unmarshal(resp.Body(), &metadata)
		if err != nil {
			return nil, fmt.Errorf("couldn't unmarshal file IDs response: %s", err)
		}

		allFiles = append(allFiles, metadata.Results...)

		if metadata.Next == "" {
			break
		}
		page++
	}

	return allFiles, nil
}
