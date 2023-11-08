package files

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/util"
)

var (
	outputDir        string
	partialNameMatch bool
)

// filesGetCmd represents the filesGet command
var filesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get files from the Trickest file storage",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fileNames := strings.Split(Files, ",")
		for _, fileName := range fileNames {
			err := getFile(fileName, outputDir, partialNameMatch)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
			} else {
				fmt.Printf("Retrieved matches for %s successfully\n", fileName)
			}
		}
	},
}

func init() {
	FilesCmd.AddCommand(filesGetCmd)

	filesGetCmd.Flags().StringVar(&outputDir, "output-dir", ".", "Path to directory which should be used to store files")

	filesGetCmd.Flags().BoolVar(&partialNameMatch, "partial-name-match", false, "Get all files with a partial name match")
}

func getFile(fileName string, outputDir string, partialNameMatch bool) error {
	metadata, err := getMetadata(fileName)
	if err != nil {
		return fmt.Errorf("couldn't search for %s: %s", fileName, err)
	}

	if len(metadata) == 0 {
		return fmt.Errorf("couldn't find any matches for %s", fileName)
	}

	matchFound := false
	for _, fileMetadata := range metadata {
		if partialNameMatch || fileMetadata.Name == fileName {
			matchFound = true
			signedURL, err := getSignedURLs(fileMetadata.ID)
			if err != nil {
				return fmt.Errorf("couldn't get a signed URL for %s: %s", fileMetadata.Name, err)
			}

			err = util.DownloadFile(signedURL, outputDir, fileMetadata.Name)
			if err != nil {
				return fmt.Errorf("couldn't download %s: %s", fileMetadata.Name, err)
			}
		}
	}

	if !matchFound {
		return fmt.Errorf("couldn't find any matches for %s", fileName)
	}
	return nil
}

func getSignedURLs(fileID string) (string, error) {
	resp := request.Trickest.Get().DoF("file/%s/signed_url/", fileID)
	if resp == nil || resp.Status() != http.StatusOK {
		return "", fmt.Errorf("unexpected response status code: %d", resp.Status())
	}
	var signedURL string

	err := json.Unmarshal(resp.Body(), &signedURL)
	if err != nil {
		return "", fmt.Errorf("couldn't unmarshal signedURL response: %s", err)
	}

	return signedURL, nil
}
