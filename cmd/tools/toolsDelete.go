package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"
)

type DeleteConfig struct {
	Token   string
	BaseURL string

	ToolID   string
	ToolName string
}

var deleteCfg = &DeleteConfig{}

func init() {
	ToolsCmd.AddCommand(toolsDeleteCmd)

	toolsDeleteCmd.Flags().StringVar(&deleteCfg.ToolID, "id", "", "ID of the tool to delete")
	toolsDeleteCmd.Flags().StringVar(&deleteCfg.ToolName, "name", "", "Name of the tool to delete")
}

var toolsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a private tool integration",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if deleteCfg.ToolName == "" && deleteCfg.ToolID == "" {
			fmt.Fprintf(os.Stderr, "Error: tool ID or name is required\n")
			os.Exit(1)
		}

		if deleteCfg.ToolID != "" && deleteCfg.ToolName != "" {
			fmt.Fprintf(os.Stderr, "Error: tool ID and name cannot both be provided\n")
			os.Exit(1)
		}

		deleteCfg.Token = util.GetToken()
		deleteCfg.BaseURL = util.Cfg.BaseUrl
		if err := runDelete(deleteCfg); err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
	},
}

func runDelete(cfg *DeleteConfig) error {
	client, err := trickest.NewClient(trickest.WithToken(cfg.Token), trickest.WithBaseURL(cfg.BaseURL))
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	var toolID uuid.UUID
	if cfg.ToolID != "" {
		toolID, err = uuid.Parse(cfg.ToolID)
		if err != nil {
			return fmt.Errorf("failed to parse tool ID: %w", err)
		}
	} else {
		tool, err := client.GetPrivateToolByName(ctx, cfg.ToolName)
		if err != nil {
			return fmt.Errorf("failed to find tool: %w", err)
		}
		toolID = *tool.ID
	}

	err = client.DeletePrivateTool(ctx, toolID)
	if err != nil {
		return fmt.Errorf("failed to delete tool: %w", err)
	}
	return nil
}
