package files

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"
)

type DeleteConfig struct {
	Token   string
	BaseURL string

	FileNames []string
}

var deleteCfg = &DeleteConfig{}

func init() {
	FilesCmd.AddCommand(filesDeleteCmd)

	filesDeleteCmd.Flags().StringSliceVar(&deleteCfg.FileNames, "file", []string{}, "File(s) to delete")
	filesDeleteCmd.MarkFlagRequired("file")
}

// filesDeleteCmd represents the filesDelete command
var filesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete files from the Trickest file storage",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		deleteCfg.Token = util.GetToken()
		deleteCfg.BaseURL = util.BaseURL
		if err := runDelete(deleteCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runDelete(cfg *DeleteConfig) error {
	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	for _, fileName := range cfg.FileNames {
		file, err := client.GetFileByName(ctx, fileName)
		if err != nil {
			return fmt.Errorf("failed to get file: %w", err)
		}

		err = client.DeleteFile(ctx, file.ID)
		if err != nil {
			return fmt.Errorf("failed to delete file: %w", err)
		}

		fmt.Printf("Deleted file %q successfully\n", fileName)
	}

	return nil
}
