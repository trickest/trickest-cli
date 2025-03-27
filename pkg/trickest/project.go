package trickest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Project represents a project
type Project struct {
	ID            *uuid.UUID `json:"id,omitempty"`
	Name          string     `json:"name,omitempty"`
	Description   string     `json:"description,omitempty"`
	SpaceID       *uuid.UUID `json:"space_info,omitempty"`
	SpaceName     string     `json:"space_name,omitempty"`
	WorkflowCount int        `json:"workflow_count,omitempty"`
	CreatedDate   *time.Time `json:"created_date,omitempty"`
	ModifiedDate  *time.Time `json:"modified_date,omitempty"`
	Author        string     `json:"author,omitempty"`
	Workflows     []Workflow `json:"workflows,omitempty"`
}

func (c *Client) CreateProject(ctx context.Context, name string, description string, spaceID uuid.UUID) (*Project, error) {
	path := fmt.Sprintf("/projects/?vault=%s", c.vaultID)

	project := Project{
		Name:        name,
		Description: description,
		SpaceID:     &spaceID,
	}

	var newProject Project
	if err := c.Hive.doJSON(ctx, http.MethodPost, path, &project, &newProject); err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return &newProject, nil
}

// DeleteProject deletes a project
func (c *Client) DeleteProject(ctx context.Context, id uuid.UUID) error {
	path := fmt.Sprintf("/projects/%s/", id)
	if err := c.Hive.doJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	return nil
}
