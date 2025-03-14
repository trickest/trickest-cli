package trickest

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
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

// GetSubJobs retrieves all sub-jobs for a run
func (c *Client) GetSubJobs(runID uuid.UUID) ([]SubJob, error) {
	var result struct {
		Results []SubJob `json:"results"`
	}
	path := fmt.Sprintf("/subjob/?execution=%s", runID.String())
	path += "&page_size=" + strconv.Itoa(math.MaxInt)

	if err := c.doJSON(context.Background(), http.MethodGet, path, nil, &result); err != nil {
		return nil, fmt.Errorf("failed to get sub-jobs: %w", err)
	}

	return result.Results, nil
}
