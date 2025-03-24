package config

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/trickest/trickest-cli/pkg/trickest"
)

// WorkflowRunSpec represents the specification for a workflow or run
type WorkflowRunSpec struct {
	// Run identification
	RunID        string
	NumberOfRuns int
	AllRuns      bool

	// Workflow identification
	SpaceName    string
	ProjectName  string
	WorkflowName string
	URL          string
}

// GetRuns retrieves runs based on the specification
func (s WorkflowRunSpec) GetRuns(ctx context.Context, client *trickest.Client) ([]trickest.Run, error) {
	// If we have an specific run ID or no multiple run flags, get a single run
	if s.RunID != "" || (s.NumberOfRuns == 0 && !s.AllRuns) {
		run, err := s.resolveSingleRun(ctx, client)
		if err != nil {
			return nil, err
		}
		return []trickest.Run{*run}, nil
	}

	// Get multiple runs from the workflow
	workflow, err := s.GetWorkflow(ctx, client)
	if err != nil {
		return nil, err
	}

	limit := 0 // 0 means get all runs
	if s.NumberOfRuns > 0 {
		limit = s.NumberOfRuns
	}

	runs, err := client.GetRuns(ctx, workflow.ID, "", limit)
	if err != nil {
		return nil, fmt.Errorf("error getting runs: %w", err)
	}
	return runs, nil
}

// GetWorkflow gets the workflow ID from the specification
func (s WorkflowRunSpec) GetWorkflow(ctx context.Context, client *trickest.Client) (*trickest.Workflow, error) {
	if s.URL != "" {
		workflow, err := client.GetWorkflowByURL(ctx, s.URL)
		if err != nil {
			return nil, fmt.Errorf("error getting workflow by URL: %w", err)
		}
		return workflow, nil
	}

	if s.SpaceName != "" && s.WorkflowName != "" {
		workflow, err := client.GetWorkflowByLocation(ctx, s.SpaceName, s.ProjectName, s.WorkflowName)
		if err != nil {
			return nil, fmt.Errorf("error getting workflow by location: %w", err)
		}
		return workflow, nil
	}

	return nil, fmt.Errorf("must provide either URL or space and workflow name to resolve workflow")
}

// resolveSingleRun resolves a single run from the specification
func (s WorkflowRunSpec) resolveSingleRun(ctx context.Context, client *trickest.Client) (*trickest.Run, error) {
	if s.RunID != "" {
		run, err := client.GetRun(ctx, uuid.MustParse(s.RunID))
		if err != nil {
			return nil, fmt.Errorf("error getting run: %w", err)
		}
		return run, nil
	}

	if s.URL != "" {
		// First try to get run ID from URL
		run, err := client.GetRunByURL(ctx, s.URL)
		if err != nil {
			return nil, fmt.Errorf("error getting run from URL run ID: %w", err)
		}
		if run != nil {
			return run, nil
		}
	}

	// If no specific run found, get the latest run from the workflow
	workflow, err := s.GetWorkflow(ctx, client)
	if err != nil {
		return nil, err
	}

	run, err := client.GetLatestRun(ctx, workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("error getting latest run: %w", err)
	}
	return run, nil
}
