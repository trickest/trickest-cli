package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/pkg/display"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"
)

type Config struct {
	Token   string
	BaseURL string

	JSONOutput bool
}

var cfg = &Config{}

var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List private tool integrations",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cfg.Token = util.GetToken()
		cfg.BaseURL = util.Cfg.BaseUrl
		if err := runList(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	ToolsCmd.AddCommand(toolsListCmd)

	toolsListCmd.Flags().BoolVar(&cfg.JSONOutput, "json", false, "Display output in JSON format")
}

func runList(cfg *Config) error {
	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()
	tools, err := client.ListPrivateTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	if len(tools) == 0 {
		return fmt.Errorf("couldn't find any private tools")
	}

	if cfg.JSONOutput {
		data, err := json.Marshal(tools)
		if err != nil {
			return fmt.Errorf("failed to marshal tools: %w", err)
		}
		fmt.Println(string(data))
	} else {
		err = display.PrintTools(os.Stdout, tools)
		if err != nil {
			return fmt.Errorf("failed to print tools: %w", err)
		}
	}

	return nil
}
