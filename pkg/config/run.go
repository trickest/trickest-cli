package config

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/trickest/trickest-cli/pkg/trickest"
)

// RunSpec represents the specification for a workflow or run
type RunSpec struct {
	// Run identification
	RunID string
	// Workflow identification
	SpaceName    string
	ProjectName  string
	WorkflowName string
	URL          string
}

// GetRun retrieves the run based on the specification, trying each method in order
func (s RunSpec) GetRun(ctx context.Context, client *trickest.Client) (*trickest.Run, error) {
	if s.RunID != "" {
		return s.getFromRunID(ctx, client)
	}

	if s.URL != "" {
		return s.getFromURL(ctx, client)
	}

	if s.SpaceName != "" && s.WorkflowName != "" {
		return s.getFromLocation(ctx, client)
	}

	return nil, fmt.Errorf("must provide either run ID, URL, or space and workflow name")
}

// getFromRunID retrieves a run directly using its ID
func (s RunSpec) getFromRunID(ctx context.Context, client *trickest.Client) (*trickest.Run, error) {
	run, err := client.GetRun(ctx, uuid.MustParse(s.RunID))
	if err != nil {
		return nil, fmt.Errorf("error getting run: %w", err)
	}
	return run, nil
}

// getFromURL retrieves a run from a URL, either directly or from the latest run of the workflow
func (s RunSpec) getFromURL(ctx context.Context, client *trickest.Client) (*trickest.Run, error) {
	// First try to get run ID from URL
	run, err := client.GetRunByURL(ctx, s.URL)
	if err != nil {
		return nil, fmt.Errorf("error getting run from URL run ID: %w", err)
	}

	if run != nil {
		return run, nil
	}

	// If no run found, get workflow from URL and get latest run
	workflow, err := client.GetWorkflowByURL(ctx, s.URL)
	if err != nil {
		return nil, fmt.Errorf("error getting workflow by URL: %w", err)
	}

	run, err = client.GetLatestRun(ctx, workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("error getting latest run: %w", err)
	}
	return run, nil
}

// getFromLocation retrieves the latest run of a workflow specified by space/project/workflow names
func (s RunSpec) getFromLocation(ctx context.Context, client *trickest.Client) (*trickest.Run, error) {
	workflow, err := client.GetWorkflowByLocation(ctx, s.SpaceName, s.ProjectName, s.WorkflowName)
	if err != nil {
		return nil, fmt.Errorf("error getting workflow by location: %w", err)
	}

	run, err := client.GetLatestRun(ctx, workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("error getting latest run: %w", err)
	}
	return run, nil
}
