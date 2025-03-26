package files

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"
)

type CreateConfig struct {
	Token   string
	BaseURL string

	FilePaths []string
}

var createCfg = &CreateConfig{}

func init() {
	FilesCmd.AddCommand(filesCreateCmd)

	filesCreateCmd.Flags().StringSliceVar(&createCfg.FilePaths, "file", []string{}, "File(s) to upload")
	filesCreateCmd.MarkFlagRequired("file")
}

// filesCreateCmd represents the filesCreate command
var filesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create files on the Trickest file storage",
	Long: "Create files on the Trickest file storage.\n" +
		"Note: If a file with the same name already exists, it will be overwritten.",
	Run: func(cmd *cobra.Command, args []string) {
		createCfg.Token = util.GetToken()
		createCfg.BaseURL = util.BaseURL
		if err := runCreate(createCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runCreate(cfg *CreateConfig) error {
	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	for _, filePath := range cfg.FilePaths {
		_, err := client.UploadFile(ctx, filePath, true)
		if err != nil {
			return fmt.Errorf("failed to upload file: %w", err)
		}
	}

	return nil
}
