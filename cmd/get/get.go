package get

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/trickest/trickest-cli/pkg/config"
	"github.com/trickest/trickest-cli/pkg/display"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
)

// Config holds the configuration for the get command
type Config struct {
	Token   string
	BaseURL string

	Watch                 bool
	IncludePrimitiveNodes bool
	JSONOutput            bool

	RunID   string
	RunSpec config.RunSpec
}

var cfg = &Config{}

func init() {
	GetCmd.Flags().BoolVar(&cfg.Watch, "watch", false, "Watch the workflow execution if it's still running")
	GetCmd.Flags().BoolVar(&cfg.IncludePrimitiveNodes, "show-params", false, "Show parameters in the workflow tree")
	GetCmd.Flags().StringVar(&cfg.RunID, "run", "", "Get the status of a specific run")
	GetCmd.Flags().BoolVar(&cfg.JSONOutput, "json", false, "Display output in JSON format")
}

// GetCmd represents the get command
var GetCmd = &cobra.Command{
	Use:   "get",
	Short: "Displays status of a workflow",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cfg.Token = util.GetToken()
		cfg.BaseURL = util.BaseURL
		cfg.RunSpec = config.RunSpec{
			RunID:        cfg.RunID,
			SpaceName:    util.SpaceName,
			ProjectName:  util.ProjectName,
			WorkflowName: util.WorkflowName,
			URL:          util.URL,
		}
		run(cfg)
	},
}

func run(cfg *Config) {
	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)

	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	ctx := context.Background()

	run, err := cfg.RunSpec.GetRun(ctx, client)
	if err != nil {
		fmt.Printf("Error getting run: %v\n", err)
		return
	}

	err = displayRunDetails(ctx, client, run, cfg)
	if err != nil {
		fmt.Printf("Error handling run output: %v\n", err)
		return
	}
}

func displayRunDetails(ctx context.Context, client *trickest.Client, run *trickest.Run, cfg *Config) error {
	if cfg.JSONOutput {
		ipAddresses, err := client.GetRunIPAddresses(ctx, *run.ID)
		if err != nil {
			fmt.Printf("Warning: Couldn't get the run IP addresses: %s", err)
		} else {
			run.IPAddresses = ipAddresses
		}

		data, err := json.MarshalIndent(run, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling run data: %w", err)
		}
		output := string(data)
		fmt.Println(output)
	} else {
		version, err := client.GetWorkflowVersion(ctx, *run.WorkflowVersionInfo)
		if err != nil {
			return fmt.Errorf("error getting workflow version: %w", err)
		}
		if cfg.Watch {
			watcher, err := display.NewRunWatcher(
				client,
				*run.ID,
				display.WithWorkflowVersion(version),
				display.WithIncludePrimitiveNodes(cfg.IncludePrimitiveNodes),
			)
			if err != nil {
				return fmt.Errorf("error creating run watcher: %w", err)
			}

			err = watcher.Watch(ctx)
			if err != nil {
				return fmt.Errorf("error watching run: %w", err)
			}
		} else {
			printer := display.NewRunPrinter(cfg.IncludePrimitiveNodes, os.Stdout)
			subjobs, err := client.GetSubJobs(*run.ID)
			if err != nil {
				return fmt.Errorf("error getting subjobs: %w", err)
			}
			printer.PrintAll(run, subjobs, version)
		}
	}
	return nil
}
