package library

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

// librarySearchCmd represents the librarySearch command
var librarySearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for workflows, modules, and tools in the Trickest library",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Please provide a search query")
			os.Exit(1)
		}
		cfg.Token = util.GetToken()
		cfg.BaseURL = util.Cfg.BaseUrl
		search := args[0]
		if err := runSearch(cfg, search); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	LibraryCmd.AddCommand(librarySearchCmd)
	librarySearchCmd.Flags().BoolVar(&cfg.JSONOutput, "json", false, "Display output in JSON format")
}

func runSearch(cfg *Config, search string) error {
	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	modules, err := client.SearchLibraryModules(ctx, search)
	if err != nil {
		return fmt.Errorf("failed to search for modules: %w", err)
	}

	workflows, err := client.SearchLibraryWorkflows(ctx, search)
	if err != nil {
		return fmt.Errorf("failed to search for workflows: %w", err)
	}

	tools, err := client.SearchLibraryTools(ctx, search)
	if err != nil {
		return fmt.Errorf("failed to search for tools: %w", err)
	}

	if cfg.JSONOutput {
		results := map[string]interface{}{
			"workflows": workflows,
			"modules":   modules,
			"tools":     tools,
		}
		data, err := json.Marshal(results)
		if err != nil {
			return fmt.Errorf("failed to marshal response data: %w", err)
		}
		fmt.Println(string(data))
	} else {
		if len(modules) > 0 {
			err = display.PrintModules(os.Stdout, modules)
			if err != nil {
				return fmt.Errorf("failed to print modules: %w", err)
			}
		}
		if len(workflows) > 0 {
			err = display.PrintWorkflows(os.Stdout, workflows)
			if err != nil {
				return fmt.Errorf("failed to print workflows: %w", err)
			}
		}
		if len(tools) > 0 {
			err = display.PrintTools(os.Stdout, tools)
			if err != nil {
				return fmt.Errorf("failed to print tools: %w", err)
			}
		}
		if len(modules) == 0 && len(workflows) == 0 && len(tools) == 0 {
			return fmt.Errorf("no results found for search query: %s", search)
		}
	}
	return nil
}
