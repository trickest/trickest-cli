package trickest

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// GetLibraryWorkflows searches for workflows in the library by name
func (c *Client) GetLibraryWorkflows(ctx context.Context, search string) ([]Workflow, error) {
	path := fmt.Sprintf("/library/workflow/?search=%s", search)

	workflows, err := GetPaginated[Workflow](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflows: %w", err)
	}

	return workflows, nil
}

// GetLibraryWorkflowByName retrieves a workflow by name from the library
func (c *Client) GetLibraryWorkflowByName(ctx context.Context, name string) (*Workflow, error) {
	workflows, err := c.GetLibraryWorkflows(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflows: %w", err)
	}

	for _, workflow := range workflows {
		if workflow.Name == name {
			return &workflow, nil
		}
	}

	return nil, fmt.Errorf("workflow %s was not found in the library", name)
}

// CopyWorkflowFromLibrary copies a workflow from the library to a space and optionally a project
// Set destinationProjectID to uuid.Nil for no project
func (c *Client) CopyWorkflowFromLibrary(ctx context.Context, workflowID uuid.UUID, destinationSpaceID uuid.UUID, destinationProjectID uuid.UUID) (Workflow, error) {
	path := fmt.Sprintf("/library/workflow/%s/copy/", workflowID)

	destination := struct {
		SpaceID   uuid.UUID  `json:"space_info"`
		ProjectID *uuid.UUID `json:"project_info,omitempty"`
	}{
		SpaceID: destinationSpaceID,
	}

	if destinationProjectID != uuid.Nil {
		destination.ProjectID = &destinationProjectID
	}

	var workflow Workflow
	if err := c.Hive.doJSON(ctx, http.MethodPost, path, destination, &workflow); err != nil {
		return Workflow{}, fmt.Errorf("failed to copy workflow: %w", err)
	}

	return workflow, nil
}
