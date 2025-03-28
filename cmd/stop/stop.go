package stop

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/pkg/config"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"
)

type Config struct {
	Token   string
	BaseURL string

	RunSpec        config.WorkflowRunSpec
	Nodes          []string
	ChildrenRanges []string
	Children       []int
}

var cfg = &Config{}

func init() {
	StopCmd.Flags().BoolVar(&cfg.RunSpec.AllRuns, "all", false, "Stop all runs")
	StopCmd.Flags().StringVar(&cfg.RunSpec.RunID, "run", "", "Stop a specific run")
	StopCmd.Flags().StringSliceVar(&cfg.Nodes, "nodes", []string{}, "Nodes to stop. If none specified, the entire run will be stopped. If a node is a task group, the `--child` flag must be used (can be used multiple times)")
	StopCmd.Flags().StringSliceVar(&cfg.ChildrenRanges, "child", []string{}, "Child tasks to stop. If a node is a task group, the `--child` flag must be used (can be used multiple times)")
}

// StopCmd represents the stop command
var StopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a workflow/node",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cfg.Token = util.GetToken()
		cfg.BaseURL = util.Cfg.BaseUrl
		cfg.RunSpec.ProjectName = util.ProjectName
		cfg.RunSpec.SpaceName = util.SpaceName
		cfg.RunSpec.WorkflowName = util.WorkflowName
		cfg.RunSpec.URL = util.URL
		if err := run(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func run(cfg *Config) error {
	if len(cfg.Nodes) == 0 {
		node, err := config.GetNodeIDFromWorkflowURL(util.URL)
		if err == nil {
			cfg.Nodes = []string{node}
		}
	}

	for _, childRange := range cfg.ChildrenRanges {
		if !strings.Contains(childRange, "-") {
			child, err := strconv.Atoi(childRange)
			if err == nil {
				cfg.Children = append(cfg.Children, child)
			}
			continue
		}

		rangeParts := strings.Split(childRange, "-")
		if len(rangeParts) != 2 {
			return fmt.Errorf("invalid child range: %s", childRange)
		}

		start, err1 := strconv.Atoi(rangeParts[0])
		end, err2 := strconv.Atoi(rangeParts[1])
		if err1 != nil || err2 != nil || start > end {
			return fmt.Errorf("invalid child range: %s", childRange)
		}

		for i := start; i <= end; i++ {
			cfg.Children = append(cfg.Children, i)
		}
	}

	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	cfg.RunSpec.RunStatus = "RUNNING"
	runs, err := cfg.RunSpec.GetRuns(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to get runs: %w", err)
	}

	var errs []error
	for _, run := range runs {
		if len(cfg.Nodes) == 0 && len(cfg.Children) == 0 {
			err := client.StopRun(ctx, *run.ID)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to stop run %s: %w", run.ID, err))
			} else {
				fmt.Printf("Successfully sent stop request for run %s\n", run.ID)
			}
			continue
		}

		subJobs, err := client.GetSubJobs(ctx, *run.ID)
		if err != nil {
			return fmt.Errorf("failed to get subjobs for run %s: %w", run.ID.String(), err)
		}

		version, err := client.GetWorkflowVersion(ctx, *run.WorkflowVersionInfo)
		if err != nil {
			return fmt.Errorf("failed to get workflow version for run %s: %w", run.ID.String(), err)
		}
		subJobs = trickest.LabelSubJobs(subJobs, *version)

		matchingSubJobs, err := trickest.FilterSubJobs(subJobs, cfg.Nodes)
		if err != nil {
			return fmt.Errorf("no running nodes matching your query were found in the run %s: %w", run.ID.String(), err)
		}

		for _, subJob := range matchingSubJobs {
			if !subJob.TaskGroup {
				if subJob.Status != "RUNNING" {
					errs = append(errs, fmt.Errorf("cannot stop node %q (%s) - current status is %q", subJob.Label, subJob.Name, subJob.Status))
					continue
				}
				err := client.StopSubJob(ctx, subJob.ID)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to stop node %q (%s) in run %s: %w", subJob.Label, subJob.Name, run.ID, err))
					continue
				} else {
					fmt.Printf("Successfully sent stop request for node %q (%s) in run %s\n", subJob.Label, subJob.Name, run.ID)
				}
			} else {
				if len(cfg.Children) == 0 {
					errs = append(errs, fmt.Errorf("node %q (%s) is a task group - use the --child flag to specify which task(s) to stop (e.g. --child 1 or --child 1-3)", subJob.Label, subJob.Name))
					continue
				}
				for _, child := range cfg.Children {
					subJobChild, err := client.GetChildSubJob(ctx, subJob.ID, child)
					if err != nil {
						errs = append(errs, fmt.Errorf("failed to get child %d of node %q (%s): %w", child, subJob.Label, subJob.Name, err))
						continue
					}

					if subJobChild.Status != "RUNNING" {
						errs = append(errs, fmt.Errorf("cannot stop child %d of node %q (%s) - current status is %q", child, subJob.Label, subJob.Name, subJobChild.Status))
						continue
					}

					err = client.StopSubJob(ctx, subJobChild.ID)
					if err != nil {
						errs = append(errs, fmt.Errorf("failed to stop child %d of node %q (%s) in run %s: %w", child, subJob.Label, subJob.Name, run.ID, err))
						continue
					} else {
						fmt.Printf("Successfully sent stop request for child %d of node %q (%s) in run %s\n", child, subJob.Label, subJob.Name, run.ID)
					}
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors encountered while sending stop requests:\n%s", strings.Join(func() []string {
			var msgs []string
			for _, err := range errs {
				msgs = append(msgs, " - "+err.Error())
			}
			return msgs
		}(), "\n"))
	}
	return nil
}
