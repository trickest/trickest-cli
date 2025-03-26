package output

import (
	"context"
	"fmt"
	"os"

	"github.com/trickest/trickest-cli/pkg/actions"
	"github.com/trickest/trickest-cli/pkg/config"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
)

var cfg = &Config{}

func init() {
	OutputCmd.Flags().StringVar(&cfg.ConfigFile, "config", "", "YAML file to determine which nodes output(s) should be downloaded")
	OutputCmd.Flags().BoolVar(&cfg.AllRuns, "all", false, "Download output data for all runs")
	OutputCmd.Flags().IntVar(&cfg.NumberOfRuns, "runs", 1, "Number of recent runs which outputs should be downloaded")
	OutputCmd.Flags().StringVar(&cfg.RunID, "run", "", "Download output data of a specific run")
	OutputCmd.Flags().StringVar(&cfg.OutputDir, "output-dir", "", "Path to directory which should be used to store outputs")
	OutputCmd.Flags().StringVar(&cfg.Nodes, "nodes", "", "A comma-separated list of nodes whose outputs should be downloaded")
	OutputCmd.Flags().StringVar(&cfg.Files, "files", "", "A comma-separated list of file names that should be downloaded from the selected node")
}

// OutputCmd represents the download command
var OutputCmd = &cobra.Command{
	Use:   "output",
	Short: "Download workflow outputs",
	Long: `This command downloads sub-job outputs of a completed workflow run.
Downloaded files will be stored into space/project/workflow/run-timestamp directory. Every node will have it's own
directory named after it's label or ID (if the label is not unique), and an optional prefix ("<num>-") if it's 
connected to a splitter.

Use raw command line arguments or a config file to specify which nodes' output you would like to fetch.
If there is no node names specified, all outputs will be downloaded.

The YAML config file should be formatted like:
   outputs:
      - foo
      - bar
`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg.Token = util.GetToken()
		cfg.BaseURL = util.BaseURL
		cfg.RunSpec = config.WorkflowRunSpec{
			RunID:        cfg.RunID,
			AllRuns:      cfg.AllRuns,
			NumberOfRuns: cfg.NumberOfRuns,
			SpaceName:    util.SpaceName,
			ProjectName:  util.ProjectName,
			WorkflowName: util.WorkflowName,
			URL:          util.URL,
		}
		if err := run(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func run(cfg *Config) error {
	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()
	runs, err := cfg.RunSpec.GetRuns(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to get runs: %w", err)
	}
	if len(runs) == 0 {
		return fmt.Errorf("no runs found for the specified workflow")
	}

	nodes := cfg.GetNodes()
	files := cfg.GetFiles()
	path := cfg.GetOutputPath()

	for _, run := range runs {
		if err := actions.DownloadRunOutput(client, &run, nodes, files, path); err != nil {
			return fmt.Errorf("failed to download run output: %w", err)
		}
	}
	return nil
}
