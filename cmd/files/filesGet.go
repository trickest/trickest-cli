package files

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/pkg/filesystem"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"
)

type GetConfig struct {
	Token   string
	BaseURL string

	FileNames        []string
	OutputDir        string
	PartialNameMatch bool
}

var getCfg = &GetConfig{}

func init() {
	FilesCmd.AddCommand(filesGetCmd)

	filesGetCmd.Flags().StringSliceVar(&getCfg.FileNames, "file", []string{}, "File(s) to download")
	filesGetCmd.MarkFlagRequired("file")
	filesGetCmd.Flags().StringVar(&getCfg.OutputDir, "output-dir", ".", "Path to directory which should be used to store files")
	filesGetCmd.Flags().BoolVar(&getCfg.PartialNameMatch, "partial-name-match", false, "Get all files with a partial name match")
}

// filesGetCmd represents the filesGet command
var filesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get files from the Trickest file storage",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		getCfg.Token = util.GetToken()
		getCfg.BaseURL = util.Cfg.BaseUrl
		if err := runGet(getCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runGet(cfg *GetConfig) error {
	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	var files []trickest.File
	for _, fileName := range cfg.FileNames {
		if cfg.PartialNameMatch {
			matchingFiles, err := client.SearchFiles(ctx, fileName)
			if err != nil {
				return fmt.Errorf("failed to search for files: %w", err)
			}
			files = append(files, matchingFiles...)
		} else {
			file, err := client.GetFileByName(ctx, fileName)
			if err != nil {
				return fmt.Errorf("failed to get file: %w", err)
			}
			files = append(files, file)
		}
	}

	for _, file := range files {
		signedURL, err := client.GetFileSignedURL(ctx, file.ID)
		if err != nil {
			return fmt.Errorf("failed to get file signed URL: %w", err)
		}

		err = filesystem.DownloadFile(signedURL, cfg.OutputDir, file.Name, true)
		if err != nil {
			return fmt.Errorf("failed to download file: %w", err)
		}
	}

	return nil
}
