package trickest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// Run represents a workflow run
type Run struct {
	ID                  *uuid.UUID         `json:"id,omitempty"`
	Name                string             `json:"name,omitempty"`
	Status              string             `json:"status,omitempty"`
	Machines            int                `json:"machines,omitempty"`
	Parallelism         int                `json:"parallelism,omitempty"`
	WorkflowVersionInfo *uuid.UUID         `json:"workflow_version_info,omitempty"`
	WorkflowInfo        *uuid.UUID         `json:"workflow_info,omitempty"`
	WorkflowName        string             `json:"workflow_name,omitempty"`
	SpaceInfo           *uuid.UUID         `json:"space_info,omitempty"`
	SpaceName           string             `json:"space_name,omitempty"`
	ProjectInfo         *uuid.UUID         `json:"project_info,omitempty"`
	ProjectName         string             `json:"project_name,omitempty"`
	CreationType        string             `json:"creation_type,omitempty"`
	CreatedDate         *time.Time         `json:"created_date,omitempty"`
	StartedDate         *time.Time         `json:"started_date,omitempty"`
	CompletedDate       *time.Time         `json:"completed_date,omitempty"`
	Finished            bool               `json:"finished,omitempty"`
	Author              string             `json:"author,omitempty"`
	Fleet               *uuid.UUID         `json:"fleet,omitempty"`
	FleetName           string             `json:"fleet_name,omitempty"`
	Vault               *uuid.UUID         `json:"vault,omitempty"`
	UseStaticIPs        *bool              `json:"use_static_ips,omitempty"`
	IPAddresses         []string           `json:"ip_addresses,omitempty"`
	RunInsights         *RunSubJobInsights `json:"run_insights,omitempty"`
	AverageDuration     *Duration          `json:"average_duration,omitempty"`
}

func (r Run) Duration() time.Duration {
	if r.StartedDate == nil {
		return 0
	}
	if r.CompletedDate == nil {
		return time.Since(*r.StartedDate)
	}
	return r.CompletedDate.Sub(*r.StartedDate)
}

// Duration is a custom type for duration that json marshals to "Xh Ym" or "Xm Ys" matching the web UI
type Duration struct {
	Duration time.Duration
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	seconds := int64(d.Duration.Seconds())
	if seconds >= 3600 {
		return json.Marshal(fmt.Sprintf("%dh %dm", seconds/3600, (seconds%3600)/60))
	}
	return json.Marshal(fmt.Sprintf("%dm %ds", seconds/60, seconds%60))
}

type RunSubJobInsights struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Stopping  int `json:"stopping"`
	Stopped   int `json:"stopped"`
}

// GetRun retrieves a run by ID
func (c *Client) GetRun(ctx context.Context, id uuid.UUID) (*Run, error) {
	var run Run
	path := fmt.Sprintf("/execution/%s/", id.String())

	if err := c.Hive.doJSON(ctx, http.MethodGet, path, nil, &run); err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}

	return &run, nil
}

// GetRunByURL retrieves a run from a workflow URL
func (c *Client) GetRunByURL(ctx context.Context, workflowURL string) (*Run, error) {
	u, err := url.Parse(workflowURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	queryParams, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("invalid URL query: %w", err)
	}

	runIDs, found := queryParams["run"]
	if !found {
		return nil, nil // No run ID present, but this isn't an error
	}

	if len(runIDs) != 1 {
		return nil, fmt.Errorf("invalid number of run parameters in URL: %d", len(runIDs))
	}

	runID, err := uuid.Parse(runIDs[0])
	if err != nil {
		return nil, fmt.Errorf("invalid run ID format: %w", err)
	}

	return c.GetRun(ctx, runID)
}

// GetRuns retrieves workflow runs with optional filtering
func (c *Client) GetRuns(ctx context.Context, workflowID uuid.UUID, status string, limit int) ([]Run, error) {
	path := fmt.Sprintf("/execution/?type=Editor&vault=%s", c.vaultID)

	if workflowID != uuid.Nil {
		path += fmt.Sprintf("&workflow=%s", workflowID)
	}

	if status != "" {
		path += fmt.Sprintf("&status=%s", status)
	}

	runs, err := GetPaginated[Run](c.Hive, ctx, path, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get runs: %w", err)
	}

	return runs, nil
}

// GetLatestRun retrieves the latest run for a workflow
func (c *Client) GetLatestRun(ctx context.Context, workflowID uuid.UUID) (*Run, error) {
	runs, err := c.GetRuns(ctx, workflowID, "", 1)
	if err != nil {
		return nil, fmt.Errorf("failed to get runs: %w", err)
	}
	if len(runs) < 1 {
		return nil, fmt.Errorf("no runs found for workflow")
	}
	return &runs[0], nil
}

// GetRunIPAddresses retrieves the IP addresses associated with a run
func (c *Client) GetRunIPAddresses(ctx context.Context, runID uuid.UUID) ([]string, error) {
	var ipAddresses []string
	path := fmt.Sprintf("/execution/%s/ips/", runID)

	if err := c.Hive.doJSON(ctx, http.MethodGet, path, nil, &ipAddresses); err != nil {
		return nil, fmt.Errorf("failed to get run IP addresses: %w", err)
	}

	return ipAddresses, nil
}

// StopRun stops a workflow run
func (c *Client) StopRun(ctx context.Context, id uuid.UUID) error {
	path := fmt.Sprintf("/execution/%s/stop/", id.String())

	if err := c.Hive.doJSON(ctx, http.MethodPost, path, nil, &Run{}); err != nil {
		return fmt.Errorf("failed to stop run: %w", err)
	}

	return nil
}

func (c *Client) CreateRun(ctx context.Context, versionID uuid.UUID, machines int, fleet Fleet, useStaticIPs bool) (*Run, error) {
	path := "/execution/"

	if versionID == uuid.Nil {
		return nil, fmt.Errorf("version ID cannot be nil")
	}

	if fleet.ID == uuid.Nil {
		return nil, fmt.Errorf("invalid fleet")
	}

	if fleet.Machines.Max == 0 {
		return nil, fmt.Errorf("fleet has no machines")
	}

	run := Run{
		WorkflowVersionInfo: &versionID,
		Fleet:               &fleet.ID,
		Vault:               &fleet.Vault,
		UseStaticIPs:        &useStaticIPs,
		Parallelism:         machines,
	}

	var createdRun Run
	if err := c.Hive.doJSON(ctx, http.MethodPost, path, &run, &createdRun); err != nil {
		return nil, fmt.Errorf("failed to create run: %w", err)
	}

	return &createdRun, nil
}

func (c *Client) GetRunSubJobInsights(ctx context.Context, runID uuid.UUID) (*RunSubJobInsights, error) {
	var insights RunSubJobInsights
	path := fmt.Sprintf("/subjob/insight?execution=%s", runID)

	if err := c.Hive.doJSON(ctx, http.MethodGet, path, nil, &insights); err != nil {
		return nil, fmt.Errorf("failed to get run insights: %w", err)
	}

	return &insights, nil
}
