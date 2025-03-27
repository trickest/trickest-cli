package delete

import (
	"context"
	"fmt"
	"os"

	"github.com/trickest/trickest-cli/pkg/config"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
)

type Config struct {
	Token   string
	BaseURL string

	WorkflowSpec config.WorkflowRunSpec
}

var cfg = &Config{}

// DeleteCmd represents the delete command
var DeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Deletes an object on the Trickest platform",
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

	// Delete only the innermost object found (workflow > project > space)
	switch {
	case workflow != nil:
		err = client.DeleteWorkflow(ctx, workflow.ID)
		if err != nil {
			return fmt.Errorf("failed to delete workflow: %w", err)
		}
	case cfg.WorkflowSpec.Project != nil:
		err = client.DeleteProject(ctx, *cfg.WorkflowSpec.Project.ID)
		if err != nil {
			return fmt.Errorf("failed to delete project: %w", err)
		}
	case cfg.WorkflowSpec.Space != nil:
		err = client.DeleteSpace(ctx, *cfg.WorkflowSpec.Space.ID)
		if err != nil {
			return fmt.Errorf("failed to delete space: %w", err)
		}
	}

	return nil
}
