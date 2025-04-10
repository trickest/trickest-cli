package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trickest/trickest-cli/pkg/trickest"
)

// WorkflowRunSpec represents the specification for a workflow or run
type WorkflowRunSpec struct {
	// Run identification
	RunID        string
	NumberOfRuns int
	AllRuns      bool
	RunStatus    string

	// Workflow identification
	SpaceName    string
	ProjectName  string
	WorkflowName string
	URL          string

	// Resolved objects
	Space   *trickest.Space
	Project *trickest.Project
}

// GetRuns retrieves runs based on the specification
func (s WorkflowRunSpec) GetRuns(ctx context.Context, client *trickest.Client) ([]trickest.Run, error) {
	// If we have an specific run ID or no multiple run flags, get a single run
	if s.RunID != "" || (s.NumberOfRuns == 0 && !s.AllRuns) || strings.Contains(s.URL, "?run=") {
		run, err := s.resolveSingleRun(ctx, client)
		if err != nil {
			return nil, err
		}
		if s.RunStatus != "" && run.Status != s.RunStatus {
			return nil, fmt.Errorf("run %s has status %q, expected status %q", run.ID, run.Status, s.RunStatus)
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

	runs, err := client.GetRuns(ctx, workflow.ID, s.RunStatus, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get runs: %w", err)
	}

	// Set the workflow name and ID for each run because the mass get doesn't include them
	for i := range runs {
		runs[i].WorkflowName = workflow.Name
		runs[i].WorkflowInfo = &workflow.ID
	}

	return runs, nil
}

// GetWorkflow gets the workflow ID from the specification
func (s WorkflowRunSpec) GetWorkflow(ctx context.Context, client *trickest.Client) (*trickest.Workflow, error) {
	var workflow *trickest.Workflow
	var err error

	if s.URL != "" {
		workflow, err = client.GetWorkflowByURL(ctx, s.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to get workflow by URL: %w", err)
		}
		return workflow, nil
	}

	if s.SpaceName != "" && s.WorkflowName != "" {
		if s.Space == nil {
			workflow, err = client.GetWorkflowByLocation(ctx, s.SpaceName, s.ProjectName, s.WorkflowName)
			if err != nil {
				return nil, fmt.Errorf("failed to get workflow by location: %w", err)
			}
		} else {
			var projectID uuid.UUID
			if s.Project != nil {
				projectID = *s.Project.ID
			}

			workflows, err := client.GetWorkflows(ctx, *s.Space.ID, projectID, s.WorkflowName)
			if err != nil {
				return nil, fmt.Errorf("failed to get workflows by location: %w", err)
			}
			if len(workflows) == 0 {
				return nil, fmt.Errorf("workflow %q not found in space %q", s.WorkflowName, s.SpaceName)
			}
			for _, wf := range workflows {
				if wf.Name == s.WorkflowName {
					workflow = &wf
					break
				}
			}
			if workflow == nil {
				return nil, fmt.Errorf("workflow %q not found in space %q", s.WorkflowName, s.SpaceName)
			}
		}
		return workflow, nil
	}

	return nil, fmt.Errorf("must provide either URL or space and workflow name to resolve workflow")
}

// CreateMissing creates a space if it doesn't exist, and optionally creates a project if one was specified and also doesn't exist
// If the space or project already exist, it will do nothing
func (s *WorkflowRunSpec) CreateMissing(ctx context.Context, client *trickest.Client) error {
	if s.SpaceName == "" {
		return fmt.Errorf("space name is required")
	}

	if s.Space == nil {
		space, err := client.CreateSpace(ctx, s.SpaceName, "")
		if err != nil {
			return fmt.Errorf("failed to create space %q: %w", s.SpaceName, err)
		}
		s.Space = space
	}

	if s.ProjectName != "" && s.Project == nil {
		project, err := client.CreateProject(ctx, s.ProjectName, "", *s.Space.ID)
		if err != nil {
			return fmt.Errorf("failed to create project %q: %w", s.ProjectName, err)
		}
		s.Project = project
	}

	return nil
}

func (s *WorkflowRunSpec) ResolveSpaceAndProject(ctx context.Context, client *trickest.Client) error {
	if s.Space == nil {
		space, err := client.GetSpaceByName(ctx, s.SpaceName)
		if err != nil {
			return fmt.Errorf("failed to get space %q: %w", s.SpaceName, err)
		}
		s.Space = space
	}

	if s.ProjectName != "" && s.Project == nil {
		project, err := s.Space.GetProjectByName(s.ProjectName)
		if err != nil {
			return fmt.Errorf("failed to get project %q: %w", s.ProjectName, err)
		}
		s.Project = project
	}
	return nil
}

// resolveSingleRun resolves a single run from the specification
func (s WorkflowRunSpec) resolveSingleRun(ctx context.Context, client *trickest.Client) (*trickest.Run, error) {
	if s.RunID != "" {
		run, err := client.GetRun(ctx, uuid.MustParse(s.RunID))
		if err != nil {
			return nil, fmt.Errorf("failed to get run: %w", err)
		}
		return run, nil
	}

	if s.URL != "" {
		// First try to get run ID from URL
		run, err := client.GetRunByURL(ctx, s.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to get run from URL: %w", err)
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
		return nil, fmt.Errorf("failed to get latest run: %w", err)
	}

	// Set the workflow name and ID for the run because the mass get doesn't include them
	run.WorkflowName = workflow.Name
	run.WorkflowInfo = &workflow.ID

	return run, nil
}
