package trickest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Tool struct {
	ID               *uuid.UUID           `json:"id,omitempty"`
	Name             string               `json:"name,omitempty"`
	Description      string               `json:"description,omitempty"`
	VaultInfo        *uuid.UUID           `json:"vault_info,omitempty"`
	Author           string               `json:"author,omitempty"`
	AuthorInfo       int                  `json:"author_info,omitempty"`
	ToolCategory     string               `json:"tool_category,omitempty"`
	ToolCategoryName string               `json:"tool_category_name,omitempty"`
	Type             string               `json:"type,omitempty"`
	Inputs           map[string]NodeInput `json:"inputs,omitempty"`
	Container        *struct {
		Args    []string `json:"args,omitempty"`
		Image   string   `json:"image,omitempty"`
		Command []string `json:"command,omitempty"`
	} `json:"container,omitempty"`
	Outputs struct {
		File   NodeOutput `json:"file,omitempty"`
		Folder NodeOutput `json:"folder,omitempty"`
	} `json:"outputs,omitempty"`
	SourceURL     string     `json:"source_url,omitempty"`
	CreatedDate   *time.Time `json:"created_date,omitempty"`
	ModifiedDate  *time.Time `json:"modified_date,omitempty"`
	OutputCommand string     `json:"output_command,omitempty"`
	LicenseInfo   struct {
		Name string `json:"name,omitempty"`
		Url  string `json:"url,omitempty"`
	} `json:"license_info,omitempty"`
	DocLink string `json:"doc_link,omitempty"`
}

type Module struct {
	ID          *uuid.UUID `json:"id,omitempty"`
	Name        string     `json:"name,omitempty"`
	Complexity  int        `json:"complexity,omitempty"`
	Description string     `json:"description,omitempty"`
	Author      string     `json:"author,omitempty"`
	CreatedDate *time.Time `json:"created_date,omitempty"`
	LibraryInfo struct {
		Community bool `json:"community,omitempty"`
		Verified  bool `json:"verified,omitempty"`
	} `json:"library_info,omitempty"`
	Data struct {
		ID      string                 `json:"id,omitempty"`
		Name    string                 `json:"name,omitempty"`
		Inputs  map[string]*NodeInput  `json:"inputs,omitempty"`
		Outputs map[string]*NodeOutput `json:"outputs,omitempty"`
		Type    string                 `json:"type,omitempty"`
	} `json:"data,omitempty"`
	Workflow string `json:"workflow,omitempty"`
}

// ListLibraryWorkflows lists all workflows in the library
func (c *Client) ListLibraryWorkflows(ctx context.Context) ([]Workflow, error) {
	path := "/library/workflow/"

	workflows, err := GetPaginated[Workflow](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflows: %w", err)
	}

	return workflows, nil
}

// SearchLibraryWorkflows searches for workflows in the library by name
func (c *Client) SearchLibraryWorkflows(ctx context.Context, search string) ([]Workflow, error) {
	path := fmt.Sprintf("/library/workflow/?search=%s", search)

	workflows, err := GetPaginated[Workflow](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflows: %w", err)
	}

	return workflows, nil
}

// GetLibraryWorkflowByName retrieves a workflow by name from the library
func (c *Client) GetLibraryWorkflowByName(ctx context.Context, name string) (*Workflow, error) {
	workflows, err := c.SearchLibraryWorkflows(ctx, name)
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

// ListLibraryTools lists all tools in the library
func (c *Client) ListLibraryTools(ctx context.Context) ([]Tool, error) {
	path := "/library/tool/"

	tools, err := GetPaginated[Tool](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools: %w", err)
	}

	return tools, nil
}

// SearchLibraryTools searches for tools in the library by a search query
func (c *Client) SearchLibraryTools(ctx context.Context, search string) ([]Tool, error) {
	path := fmt.Sprintf("/library/tool/?search=%s", search)

	tools, err := GetPaginated[Tool](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools: %w", err)
	}

	return tools, nil
}

// GetLibraryToolByName retrieves a tool by name from the library
func (c *Client) GetLibraryToolByName(ctx context.Context, name string) (*Tool, error) {
	path := fmt.Sprintf("/library/tool/?name=%s", name)

	tools, err := GetPaginated[Tool](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool: %w", err)
	}

	if len(tools) == 0 {
		return nil, fmt.Errorf("tool %s was not found in the library", name)
	}

	return &tools[0], nil
}

// ListLibraryModules lists all modules in the library
func (c *Client) ListLibraryModules(ctx context.Context) ([]Module, error) {
	path := "/library/module/"

	modules, err := GetPaginated[Module](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get modules: %w", err)
	}

	return modules, nil
}

// SearchLibraryModules searches for modules in the library by a search query
func (c *Client) SearchLibraryModules(ctx context.Context, search string) ([]Module, error) {
	path := fmt.Sprintf("/library/module/?search=%s", search)

	modules, err := GetPaginated[Module](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get modules: %w", err)
	}

	return modules, nil
}
