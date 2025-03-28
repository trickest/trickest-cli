package trickest

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
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

	subjobs, err := GetPaginated[SubJob](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-jobs: %w", err)
	}

	return subjobs, nil
}

// StopSubJob stops a sub-job
func (c *Client) StopSubJob(ctx context.Context, subJobID uuid.UUID) error {
	path := fmt.Sprintf("/subjob/%s/stop/", subJobID)

	if err := c.Hive.doJSON(ctx, http.MethodPost, path, nil, nil); err != nil {
		return fmt.Errorf("failed to stop sub-job: %w", err)
	}

	return nil
}

// GetChildSubJobs retrieves all child sub-jobs for a lifted sub-job (task group)
func (c *Client) GetChildSubJobs(ctx context.Context, parentID uuid.UUID) ([]SubJob, error) {
	path := fmt.Sprintf("/subjob/children/?parent=%s", parentID.String())

	children, err := GetPaginated[SubJob](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get child sub-jobs: %w", err)
	}

	return children, nil
}

// GetChildSubJob retrieves a specific child sub-job by task index
func (c *Client) GetChildSubJob(ctx context.Context, parentID uuid.UUID, taskIndex int) (SubJob, error) {
	path := fmt.Sprintf("/subjob/children/?parent=%s&task_index=%d", parentID.String(), taskIndex)

	children, err := GetPaginated[SubJob](c.Hive, ctx, path, 1)
	if err != nil {
		return SubJob{}, fmt.Errorf("failed to get child sub-job: %w", err)
	}

	if len(children) == 0 {
		return SubJob{}, fmt.Errorf("no child sub-job found for task index %d", taskIndex)
	}

	return children[0], nil
}

func (c *Client) GetSubJobOutputs(ctx context.Context, subJobID uuid.UUID) ([]SubJobOutput, error) {
	path := fmt.Sprintf("/subjob-output/?subjob=%s", subJobID.String())

	subJobOutputs, err := GetPaginated[SubJobOutput](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-job outputs: %w", err)
	}

	return subJobOutputs, nil
}

func (c *Client) GetModuleSubJobOutputs(ctx context.Context, moduleName string, runID uuid.UUID) ([]SubJobOutput, error) {
	path := fmt.Sprintf("/subjob-output/module-outputs/?module_name=%s&execution=%s", moduleName, runID.String())

	subJobOutputs, err := GetPaginated[SubJobOutput](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get module sub-job outputs: %w", err)
	}

	return subJobOutputs, nil
}

func (c *Client) GetOutputSignedURL(ctx context.Context, outputID uuid.UUID) (SignedURL, error) {
	path := fmt.Sprintf("/job/%s/signed_url/", outputID)

	var signedURL SignedURL
	if err := c.Orchestrator.doJSON(ctx, http.MethodGet, path, nil, &signedURL); err != nil {
		return SignedURL{}, fmt.Errorf("failed to get output signed URL: %w", err)
	}

	return signedURL, nil
}

func LabelSubJobs(subJobs []SubJob, version WorkflowVersion) []SubJob {
	labels := make(map[string]bool)
	for i := range subJobs {
		subJobs[i].Label = version.Data.Nodes[subJobs[i].Name].Meta.Label
		subJobs[i].Label = strings.ReplaceAll(subJobs[i].Label, "/", "-")
		if labels[subJobs[i].Label] {
			existingLabel := subJobs[i].Label
			subJobs[i].Label = subJobs[i].Name
			if labels[subJobs[i].Label] {
				subJobs[i].Label += "-1"
				for c := 1; c >= 1; c++ {
					if labels[subJobs[i].Label] {
						subJobs[i].Label = strings.TrimSuffix(subJobs[i].Label, "-"+strconv.Itoa(c))
						subJobs[i].Label += "-" + strconv.Itoa(c+1)
					} else {
						labels[subJobs[i].Label] = true
						break
					}
				}
			} else {
				for s := 0; s < i; s++ {
					if subJobs[s].Label == existingLabel {
						subJobs[s].Label = subJobs[s].Name
						if subJobs[s].Children != nil {
							for j := range subJobs[s].Children {
								subJobs[s].Children[j].Label = strconv.Itoa(subJobs[s].Children[j].TaskIndex) + "-" + subJobs[s].Name
							}
						}
					}
				}
				labels[subJobs[i].Label] = true
			}
		} else {
			labels[subJobs[i].Label] = true
		}
	}
	return subJobs
}

// FilterSubJobs filters the subjobs based on the identifiers (label or name (node ID))
func FilterSubJobs(subJobs []SubJob, identifiers []string) ([]SubJob, error) {
	if len(identifiers) == 0 {
		return subJobs, nil
	}

	var foundNodes []string
	var matchingSubJobs []SubJob

	for _, subJob := range subJobs {
		labelExists := slices.Contains(identifiers, subJob.Label)
		nameExists := slices.Contains(identifiers, subJob.Name)

		if labelExists {
			foundNodes = append(foundNodes, subJob.Label)
		}
		if nameExists {
			foundNodes = append(foundNodes, subJob.Name)
		}

		if labelExists || nameExists {
			matchingSubJobs = append(matchingSubJobs, subJob)
		}
	}

	for _, identifier := range identifiers {
		if !slices.Contains(foundNodes, identifier) {
			return nil, fmt.Errorf("subjob with name or label %s not found", identifier)
		}
	}

	return matchingSubJobs, nil
}
