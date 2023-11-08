package files

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/client/request"
)

// filesDeleteCmd represents the filesDelete command
var filesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete files from the Trickest file storage",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fileNames := strings.Split(Files, ",")
		for _, fileName := range fileNames {
			err := deleteFile(fileName)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
			} else {
				fmt.Printf("Deleted %s successfully\n", fileName)
			}
		}
	},
}

func init() {
	FilesCmd.AddCommand(filesDeleteCmd)
}

func deleteFile(fileName string) error {
	metadata, err := getMetadata(fileName)
	if err != nil {
		return fmt.Errorf("couldn't search for %s: %s", fileName, err)
	}

	if len(metadata) == 0 {
		return fmt.Errorf("couldn't find any matches for %s", fileName)
	}

	matchFound := false
	for _, fileMetadata := range metadata {
		if fileMetadata.Name == fileName {
			matchFound = true
			err := deleteFileByID(fileMetadata.ID)
			if err != nil {
				return fmt.Errorf("couldn't delete %s: %s", fileMetadata.Name, err)
			}
		}
	}

	if !matchFound {
		return fmt.Errorf("couldn't find any matches for %s", fileName)
	}

	return nil
}

func deleteFileByID(fileID string) error {
	resp := request.Trickest.Delete().DoF("file/%s/", fileID)
	if resp == nil || resp.Status() != http.StatusNoContent {
		return fmt.Errorf("unexpected response status code: %d", resp.Status())
	}

	return nil
}
