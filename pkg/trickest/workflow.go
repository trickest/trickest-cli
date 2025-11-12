package trickest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Workflow represents a workflow
type Workflow struct {
	ID               uuid.UUID     `json:"id,omitempty"`
	Name             string        `json:"name,omitempty"`
	Description      string        `json:"description,omitempty"`
	LongDescription  string        `json:"long_description,omitempty"`
	SpaceInfo        *uuid.UUID    `json:"space_info,omitempty"`
	SpaceName        string        `json:"space_name,omitempty"`
	ProjectInfo      *uuid.UUID    `json:"project_info,omitempty"`
	ProjectName      string        `json:"project_name,omitempty"`
	ModifiedDate     *time.Time    `json:"modified_date,omitempty"`
	CreatedDate      time.Time     `json:"created_date,omitempty"`
	ScheduleInfo     *ScheduleInfo `json:"schedule_info,omitempty"`
	WorkflowCategory string        `json:"workflow_category,omitempty"`
	Author           string        `json:"author,omitempty"`
	Executing        bool          `json:"executing,omitempty"`
}

// ScheduleInfo represents a schedule info
type ScheduleInfo struct {
	ID           string     `json:"id,omitempty"`
	Vault        string     `json:"vault,omitempty"`
	Date         *time.Time `json:"date,omitempty"`
	Workflow     string     `json:"workflow,omitempty"`
	RepeatPeriod int        `json:"repeat_period,omitempty"`
	Parallelism  int        `json:"parallelism,omitempty"`
}

// WorkflowVersion represents a workflow version
type WorkflowVersion struct {
	ID           uuid.UUID           `json:"id"`
	WorkflowInfo uuid.UUID           `json:"workflow_info"`
	Name         *string             `json:"name,omitempty"`
	Description  string              `json:"description"`
	CreatedDate  time.Time           `json:"created_date"`
	RunCount     int                 `json:"run_count"`
	Snapshot     bool                `json:"snapshot"`
	Data         WorkflowVersionData `json:"data"`
}

// WorkflowVersionData represents a workflow version data
type WorkflowVersionData struct {
	Nodes          map[string]*Node          `json:"nodes"`
	Connections    []Connection              `json:"connections"`
	PrimitiveNodes map[string]*PrimitiveNode `json:"primitiveNodes"`
	Annotations    map[string]*Annotation    `json:"annotations,omitempty"`
}

// Node represents a workflow node
type Node struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Meta struct {
		Label       string `json:"label"`
		Coordinates struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"coordinates"`
	} `json:"meta"`
	Type   string                `json:"type"`
	Inputs map[string]*NodeInput `json:"inputs"`
	Script *struct {
		Args   []any  `json:"args"`
		Image  string `json:"image"`
		Source string `json:"source"`
	} `json:"script,omitempty"`
	Outputs   map[string]*NodeOutput `json:"outputs"`
	BeeType   string                 `json:"bee_type"`
	Container *struct {
		Args    []string `json:"args,omitempty"`
		Image   string   `json:"image"`
		Command []string `json:"command"`
	} `json:"container,omitempty"`
	OutputCommand   *string `json:"output_command,omitempty"`
	WorkerConnected *string `json:"workerConnected,omitempty"`
	Workflow        *string `json:"workflow,omitempty"`
}

// NodeInput represents a node input
type NodeInput struct {
	Type            string  `json:"type"`
	Order           int     `json:"order"`
	Name            string  `json:"name,omitempty"`
	Value           any     `json:"value,omitempty"`
	Command         *string `json:"command,omitempty"`
	Description     *string `json:"description,omitempty"`
	WorkerConnected *bool   `json:"workerConnected,omitempty"`
	Multi           *bool   `json:"multi,omitempty"`
	Visible         *bool   `json:"visible,omitempty"`
}

// NodeOutput represents a node output
type NodeOutput struct {
	Type          string  `json:"type"`
	Order         int     `json:"order"`
	ParameterName *string `json:"parameter_name,omitempty"`
	Visible       *bool   `json:"visible,omitempty"`
}

// Connection represents a connection between nodes
type Connection struct {
	Source      ConnectionEndpoint `json:"source"`
	Destination ConnectionEndpoint `json:"destination"`
}

type ConnectionEndpoint struct {
	ID string `json:"id"`
}

// PrimitiveNode represents a primitive node
type PrimitiveNode struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Value       any    `json:"value"`
	TypeName    string `json:"type_name"`
	Coordinates struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"coordinates"`
	ParamName  *string `json:",omitempty"`
	UpdateFile *bool   `json:",omitempty"`
}

// Annotation represents an annotation in the workflow
type Annotation struct {
	Content     string  `json:"content"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Name        string  `json:"name"`
	Coordinates struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"coordinates"`
}

// GetWorkflow retrieves a workflow by ID
func (c *Client) GetWorkflow(ctx context.Context, id uuid.UUID) (*Workflow, error) {
	var workflow Workflow
	path := fmt.Sprintf("/workflow/%s/", id.String())

	if err := c.Hive.doJSON(ctx, http.MethodGet, path, nil, &workflow); err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return &workflow, nil
}

// GetWorkflowByURL retrieves a workflow from a URL
func (c *Client) GetWorkflowByURL(ctx context.Context, workflowURL string) (*Workflow, error) {
	u, err := url.Parse(workflowURL)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow URL: %w", err)
	}

	if !strings.HasPrefix(u.Path, "/editor/") {
		return nil, fmt.Errorf("invalid workflow URL: The URL path must start with /editor/")
	}

	// Extract workflow ID from URL
	parts := strings.Split(u.Path, "/")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid workflow URL: The URL must contain a workflow ID")
	}

	workflowID, err := uuid.Parse(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid workflow ID in URL: %w", err)
	}

	return c.GetWorkflow(ctx, workflowID)
}

// RenameWorkflow renames a workflow
func (c *Client) RenameWorkflow(ctx context.Context, workflowID uuid.UUID, newName string) (*Workflow, error) {
	path := fmt.Sprintf("/workflow/%s/", workflowID)

	workflow := Workflow{
		Name: newName,
	}

	var updatedWorkflow Workflow
	if err := c.Hive.doJSON(ctx, http.MethodPatch, path, workflow, &updatedWorkflow); err != nil {
		return nil, fmt.Errorf("failed to rename workflow: %w", err)
	}

	return &updatedWorkflow, nil
}

// GetWorkflows retrieves workflows filtered by space ID, project ID and search term
func (c *Client) GetWorkflows(ctx context.Context, spaceID uuid.UUID, projectID uuid.UUID, workflowSearchQuery string) ([]Workflow, error) {
	path := fmt.Sprintf("/workflow/?space=%s", spaceID)
	if projectID != uuid.Nil {
		path += fmt.Sprintf("&project=%s", projectID)
	}
	if workflowSearchQuery != "" {
		path += fmt.Sprintf("&search=%s", url.QueryEscape(workflowSearchQuery))
	}

	workflows, err := GetPaginated[Workflow](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflows: %w", err)
	}

	return workflows, nil
}

// GetWorkflowByLocation retrieves a workflow by its location in the space/project hierarchy
func (c *Client) GetWorkflowByLocation(ctx context.Context, spaceName, projectName, workflowName string) (*Workflow, error) {
	if spaceName == "" {
		return nil, fmt.Errorf("space name cannot be empty")
	}
	if workflowName == "" {
		return nil, fmt.Errorf("workflow name cannot be empty")
	}

	space, err := c.GetSpaceByName(ctx, spaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get space: %w", err)
	}

	var projectID uuid.UUID
	if projectName != "" {
		for _, project := range space.Projects {
			if project.Name == projectName {
				projectID = *project.ID
				break
			}
		}
		if projectID == uuid.Nil {
			return nil, fmt.Errorf("project %q not found in space %q", projectName, spaceName)
		}
	}

	workflows, err := c.GetWorkflows(ctx, *space.ID, projectID, workflowName)
	if err != nil {
		return nil, err
	}

	if len(workflows) == 0 {
		return nil, fmt.Errorf("workflow %q not found in space %q", workflowName, spaceName)
	}

	for _, wf := range workflows {
		if wf.Name == workflowName {
			return &wf, nil
		}
	}

	return nil, fmt.Errorf("workflow %q not found in space %q", workflowName, spaceName)
}

// DeleteWorkflow deletes a workflow
func (c *Client) DeleteWorkflow(ctx context.Context, id uuid.UUID) error {
	path := fmt.Sprintf("/workflow/%s/", id.String())
	if err := c.Hive.doJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	return nil
}

// GetWorkflowVersion retrieves a workflow version by ID
func (c *Client) GetWorkflowVersion(ctx context.Context, id uuid.UUID) (*WorkflowVersion, error) {
	var version WorkflowVersion
	path := fmt.Sprintf("/workflow-version/%s/", id.String())

	if err := c.Hive.doJSON(ctx, http.MethodGet, path, nil, &version); err != nil {
		return nil, fmt.Errorf("failed to get workflow version: %w", err)
	}

	return &version, nil
}

// GetLatestWorkflowVersion retrieves the latest version of a workflow
func (c *Client) GetLatestWorkflowVersion(ctx context.Context, workflowID uuid.UUID) (*WorkflowVersion, error) {
	var version WorkflowVersion
	path := fmt.Sprintf("/workflow-version/latest/?workflow=%s", workflowID)

	if err := c.Hive.doJSON(ctx, http.MethodGet, path, nil, &version); err != nil {
		return nil, fmt.Errorf("failed to get latest workflow version: %w", err)
	}

	return &version, nil
}

// GetWorkflowVersionMaxMachines retrieves the maximum machines for a workflow version
func (c *Client) GetWorkflowVersionMaxMachines(ctx context.Context, versionID uuid.UUID, fleetID uuid.UUID) (int, error) {
	var parallelism struct {
		Parallelism int `json:"parallelism"`
	}
	path := fmt.Sprintf("/workflow-version/%s/max-machines/?fleet=%s", versionID, fleetID)

	if err := c.Hive.doJSON(ctx, http.MethodGet, path, nil, &parallelism); err != nil {
		return 0, fmt.Errorf("failed to get workflow version max machines: %w", err)
	}
	if parallelism.Parallelism <= 0 {
		return 0, fmt.Errorf("invalid max machines value: %d", parallelism.Parallelism)
	}
	return parallelism.Parallelism, nil
}

func (c *Client) CreateWorkflowVersion(ctx context.Context, version *WorkflowVersion) (*WorkflowVersion, error) {
	var newVersion WorkflowVersion
	path := "/workflow-version/"
	if err := c.Hive.doJSON(ctx, http.MethodPost, path, version, &newVersion); err != nil {
		return nil, fmt.Errorf("failed to create workflow version: %w", err)
	}

	return &newVersion, nil
}

func (c *Client) GetWorkflowRunsAverageDuration(ctx context.Context, workflowID uuid.UUID) (time.Duration, error) {
	pastRuns, err := c.GetRuns(ctx, workflowID, "COMPLETED", 0)
	if err != nil {
		return 0, fmt.Errorf("failed to get past runs: %w", err)
	}

	if len(pastRuns) == 0 {
		return 0, fmt.Errorf("no past runs found for workflow")
	}

	totalDuration := time.Duration(0)
	for _, pastRun := range pastRuns {
		duration := pastRun.CompletedDate.Sub(*pastRun.StartedDate)
		totalDuration += duration
	}
	averageDuration := totalDuration / time.Duration(len(pastRuns))

	return averageDuration, nil
}
