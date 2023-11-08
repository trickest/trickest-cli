package files

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/util"
)

// filesCreateCmd represents the filesCreate command
var filesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create files on the Trickest file storage",
	Long: "Create files on the Trickest file storage.\n" +
		"Note: If a file with the same name already exists, it will be overwritten.",
	Run: func(cmd *cobra.Command, args []string) {
		filePaths := strings.Split(FileNames, ",")
		for _, filePath := range filePaths {
			err := createFile(filePath)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
			} else {
				fmt.Printf("Uploaded %s successfully\n", filePath)
			}
		}
	},
}

func init() {
	FilesCmd.AddCommand(filesCreateCmd)
}

func createFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("couldn't open %s: %s", filePath, err)
	}
	defer file.Close()

	fileName := filepath.Base(file.Name())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	defer writer.Close()

	part, err := writer.CreateFormFile("thumb", fileName)
	if err != nil {
		return fmt.Errorf("couldn't create form file for %s: %s", filePath, err)
	}

	fileInfo, _ := file.Stat()
	bar := progressbar.NewOptions64(
		fileInfo.Size(),
		progressbar.OptionSetDescription(fmt.Sprintf("Creating %s...", fileName)),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() { fmt.Println() }),
	)

	_, err = io.Copy(io.MultiWriter(part, bar), file)
	if err != nil {
		return fmt.Errorf("couldn't process %s: %s", filePath, err)
	}

	_, err = part.Write([]byte("\n--" + writer.Boundary() + "--"))
	if err != nil {
		return fmt.Errorf("couldn't upload %s: %s", filePath, err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", util.Cfg.BaseUrl+"v1/file/", body)
	if err != nil {
		return fmt.Errorf("couldn't create request for %s: %s", filePath, err)
	}

	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Content-Type", writer.FormDataContentType())

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't upload %s: %s", filePath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code while uploading %s: %s", filePath, resp.Status)
	}
	return nil
}
