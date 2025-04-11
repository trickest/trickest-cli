package trickest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Space represents a space
type Space struct {
	ID             *uuid.UUID `json:"id,omitempty"`
	Name           string     `json:"name,omitempty"`
	Description    string     `json:"description,omitempty"`
	VaultID        *uuid.UUID `json:"vault_info,omitempty"`
	Playground     bool       `json:"playground,omitempty"`
	Projects       []Project  `json:"projects,omitempty"`
	ProjectsCount  int        `json:"projects_count,omitempty"`
	Workflows      []Workflow `json:"workflows,omitempty"`
	WorkflowsCount int        `json:"workflows_count,omitempty"`
	CreatedDate    *time.Time `json:"created_date,omitempty"`
	ModifiedDate   *time.Time `json:"modified_date,omitempty"`
}

// GetSpace retrieves a space by ID
func (c *Client) GetSpace(ctx context.Context, id uuid.UUID) (*Space, error) {
	var space Space
	path := fmt.Sprintf("/spaces/%s/", id)
	if err := c.Hive.doJSON(ctx, http.MethodGet, path, nil, &space); err != nil {
		return nil, fmt.Errorf("failed to get space: %w", err)
	}

	return &space, nil
}

// GetSpaces retrieves all spaces for the current vault
func (c *Client) GetSpaces(ctx context.Context, name string) ([]Space, error) {
	path := fmt.Sprintf("/spaces/?vault=%s", c.vaultID)
	if name != "" {
		path += fmt.Sprintf("&name=%s", name)
	}

	spaces, err := GetPaginated[Space](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get spaces: %w", err)
	}

	return spaces, nil
}

// GetSpaceByName retrieves a space by name
func (c *Client) GetSpaceByName(ctx context.Context, name string) (*Space, error) {
	spaces, err := c.GetSpaces(ctx, name)
	if err != nil {
		return nil, err
	}

	if len(spaces) == 0 {
		return nil, fmt.Errorf("space %q not found", name)
	}

	// loop through the results to find the space with the exact name
	for _, space := range spaces {
		if space.Name == name {
			space, err := c.GetSpace(ctx, *space.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get space: %w", err)
			}
			return space, nil
		}
	}

	return nil, fmt.Errorf("space %q not found", name)
}

// CreateSpace creates a new space
func (c *Client) CreateSpace(ctx context.Context, name string, description string) (*Space, error) {
	path := "/spaces/"

	space := Space{
		Name:        name,
		Description: description,
		VaultID:     &c.vaultID,
	}

	var newSpace Space
	if err := c.Hive.doJSON(ctx, http.MethodPost, path, &space, &newSpace); err != nil {
		return nil, fmt.Errorf("failed to create space: %w", err)
	}

	return &newSpace, nil
}

// GetProjectByName retrieves a project by name from a space
func (s *Space) GetProjectByName(name string) (*Project, error) {
	for _, project := range s.Projects {
		if project.Name == name {
			return &project, nil
		}
	}
	return nil, fmt.Errorf("project %q not found in space %q", name, s.Name)
}

// DeleteSpace deletes a space
func (c *Client) DeleteSpace(ctx context.Context, id uuid.UUID) error {
	path := fmt.Sprintf("/spaces/%s/", id)
	if err := c.Hive.doJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete space: %w", err)
	}

	return nil
}
