package list

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

type Config struct {
	Token   string
	BaseURL string

	WorkflowSpec config.WorkflowRunSpec

	JSONOutput bool
}

var cfg = &Config{}

func init() {
	ListCmd.Flags().BoolVar(&cfg.JSONOutput, "json", false, "Display output in JSON format")
}

// ListCmd represents the list command
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists objects on the Trickest platform",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cfg.Token = util.GetToken()
		cfg.BaseURL = util.Cfg.BaseUrl
		cfg.WorkflowSpec = config.WorkflowRunSpec{
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

	if cfg.WorkflowSpec.SpaceName == "" && cfg.WorkflowSpec.ProjectName == "" && cfg.WorkflowSpec.WorkflowName == "" && cfg.WorkflowSpec.URL == "" {
		spaces, err := client.GetSpaces(ctx, "")
		if err != nil {
			return fmt.Errorf("failed to get spaces: %w", err)
		}
		if cfg.JSONOutput {
			data, err := json.Marshal(spaces)
			if err != nil {
				return fmt.Errorf("failed to marshal spaces: %w", err)
			}
			fmt.Println(string(data))
		} else {
			display.PrintSpaces(os.Stdout, spaces)
		}
		return nil
	}

	if err := cfg.WorkflowSpec.ResolveSpaceAndProject(ctx, client); err != nil {
		return fmt.Errorf("failed to get space/project: %w", err)
	}

	var workflow *trickest.Workflow
	if cfg.WorkflowSpec.WorkflowName != "" || cfg.WorkflowSpec.URL != "" {
		workflow, err = cfg.WorkflowSpec.GetWorkflow(ctx, client)
		if err != nil {
			return fmt.Errorf("failed to get workflow: %w", err)
		}
	}

	var output any
	if workflow != nil {
		output = workflow
	} else if cfg.WorkflowSpec.Project != nil {
		output = cfg.WorkflowSpec.Project
	} else if cfg.WorkflowSpec.Space != nil {
		output = cfg.WorkflowSpec.Space
	}

	if project, ok := output.(*trickest.Project); ok {
		workflows, err := client.GetWorkflows(ctx, *cfg.WorkflowSpec.Space.ID, *project.ID, "")
		if err != nil {
			return fmt.Errorf("failed to get project workflows: %w", err)
		}
		project.Workflows = workflows
		output = project
	}

	if cfg.JSONOutput {
		data, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	switch v := output.(type) {
	case *trickest.Workflow:
		err = display.PrintWorkflow(os.Stdout, *v)
	case *trickest.Project:
		err = display.PrintProject(os.Stdout, *v)
	case *trickest.Space:
		err = display.PrintSpace(os.Stdout, *v)
	}

	if err != nil {
		return fmt.Errorf("failed to print object: %w", err)
	}

	return nil
}
