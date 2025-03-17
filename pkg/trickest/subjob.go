package trickest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// SubJob represents a sub-job in a workflow run
type SubJob struct {
	ID            uuid.UUID      `json:"id"`
	Status        string         `json:"status"`
	Name          string         `json:"name"`
	OutputsStatus string         `json:"outputs_status"`
	Finished      bool           `json:"finished"`
	StartedDate   time.Time      `json:"started_at"`
	FinishedDate  time.Time      `json:"finished_at"`
	Params        map[string]any `json:"params"`
	Message       string         `json:"message"`
	TaskGroup     bool           `json:"task_group"`
	TaskIndex     int            `json:"task_index"`
	Label         string
	Children      []SubJob
	TaskCount     int
}

type SubJobOutput struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Size       int       `json:"size"`
	PrettySize string    `json:"pretty_size"`
	Format     string    `json:"format"`
	Path       string    `json:"path"`
	SignedURL  string    `json:"signed_url,omitempty"`
}

type SignedURL struct {
	Url        string `json:"url"`
	Size       int    `json:"size"`
	PrettySize string `json:"pretty_size"`
}

// GetSubJobs retrieves all sub-jobs for a run
func (c *Client) GetSubJobs(ctx context.Context, runID uuid.UUID) ([]SubJob, error) {
	path := fmt.Sprintf("/subjob/?execution=%s", runID.String())

	subjobs, err := GetPaginated[SubJob](c, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-jobs: %w", err)
	}

	return subjobs, nil
}

// GetChildSubJobs retrieves all child sub-jobs for a lifted sub-job (task group)
func (c *Client) GetChildSubJobs(ctx context.Context, parentID uuid.UUID) ([]SubJob, error) {
	path := fmt.Sprintf("/subjob/children/?parent=%s", parentID.String())

	children, err := GetPaginated[SubJob](c, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get child sub-jobs: %w", err)
	}

	return children, nil
}

// GetChildSubJob retrieves a specific child sub-job by task index
func (c *Client) GetChildSubJob(ctx context.Context, parentID uuid.UUID, taskIndex int) (SubJob, error) {
	path := fmt.Sprintf("/subjob/children/?parent=%s&task_index=%d", parentID.String(), taskIndex)

	children, err := GetPaginated[SubJob](c, ctx, path, 1)
	if err != nil {
		return SubJob{}, fmt.Errorf("failed to get child sub-job: %w", err)
	}

	if len(children) == 0 {
		return SubJob{}, fmt.Errorf("no child sub-job found for task index %d", taskIndex)
	}

	return children[0], nil
}

func (c *Client) GetSubJobOutputs(ctx context.Context, subJobID uuid.Domain) ([]SubJobOutput, error) {
	path := fmt.Sprintf("/subjob-output/?subjob=%s", subJobID.String())

	subJobOutputs, err := GetPaginated[SubJobOutput](c, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-job outputs: %w", err)
	}

	return subJobOutputs, nil
}

func (c *Client) GetModuleSubJobOutputs(ctx context.Context, moduleName string, runID uuid.UUID) ([]SubJobOutput, error) {
	path := fmt.Sprintf("/subjob-output/module-outputs/?module_name=%s&execution=%s", moduleName, runID.String())

	subJobOutputs, err := GetPaginated[SubJobOutput](c, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get module sub-job outputs: %w", err)
	}

	return subJobOutputs, nil
}

func (c *Client) GetOutputSignedURL(ctx context.Context, outputID uuid.UUID) (SignedURL, error) {
	path := fmt.Sprintf("/subjob-output/%s/signed_url/", outputID)

	var signedURL SignedURL
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &signedURL); err != nil {
		return SignedURL{}, fmt.Errorf("failed to get output signed URL: %w", err)
	}

	return signedURL, nil
}
