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
		fileNames := strings.Split(FileNames, ",")
		for _, fileName := range fileNames {
			deleteFile(fileName)
		}
	},
}

func init() {
	FilesCmd.AddCommand(filesDeleteCmd)
}

func deleteFile(fileName string) {
	metadata, err := getMetadata(fileName)
	if err != nil {
		fmt.Printf("Error: couldn't search for %s: %s\n", fileName, err)
	}

	if len(metadata) == 0 {
		fmt.Printf("Error: couldn't find any matches for %s\n", fileName)
	}

	matchFound := false
	for _, fileMetadata := range metadata {
		if fileMetadata.Name == fileName {
			matchFound = true
			err := deleteFileByID(fileMetadata.ID)
			if err != nil {
				fmt.Printf("couldn't delete %s: %s\n", fileMetadata.Name, err)
			} else {
				fmt.Printf("Deleted %s\n", fileMetadata.Name)
			}
		}
	}

	if !matchFound {
		fmt.Printf("Error: couldn't find any matches for %s\n", fileName)
	}
}

func deleteFileByID(fileID string) error {
	resp := request.Trickest.Delete().DoF("file/%s/", fileID)
	if resp == nil || resp.Status() != http.StatusNoContent {
		return fmt.Errorf("unexpected response status code: %d", resp.Status())
	}

	return nil
}
