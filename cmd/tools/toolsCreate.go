package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"
	"gopkg.in/yaml.v3"
)

type CreateConfig struct {
	Token   string
	BaseURL string

	FilePath string
}

var createCfg = &CreateConfig{}

func init() {
	ToolsCmd.AddCommand(toolsCreateCmd)

	toolsCreateCmd.Flags().StringVar(&createCfg.FilePath, "file", "", "YAML file for tool definition")
	toolsCreateCmd.MarkFlagRequired("file")
}

var toolsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new private tool integration",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		createCfg.Token = util.GetToken()
		createCfg.BaseURL = util.Cfg.BaseUrl
		if err := runCreate(createCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runCreate(cfg *CreateConfig) error {
	data, err := os.ReadFile(cfg.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", cfg.FilePath, err)
	}

	client, err := trickest.NewClient(trickest.WithToken(cfg.Token), trickest.WithBaseURL(cfg.BaseURL))
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	var toolImportRequest trickest.ToolImport
	err = yaml.Unmarshal(data, &toolImportRequest)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", cfg.FilePath, err)
	}

	_, err = client.CreatePrivateTool(ctx, &toolImportRequest)
	if err != nil {
		return fmt.Errorf("failed to create tool: %w", err)
	}
	return nil
}
