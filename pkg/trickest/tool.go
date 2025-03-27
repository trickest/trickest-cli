package trickest

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

var toolOutputTypes = map[string]string{
	"file":   "2",
	"folder": "3",
}

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

type ToolImport struct {
	VaultInfo     *uuid.UUID           `json:"vault_info" yaml:"vault_info"`
	Name          string               `json:"name" yaml:"name"`
	Description   string               `json:"description" yaml:"description"`
	Category      string               `json:"tool_category_name" yaml:"category"`
	CategoryID    *uuid.UUID           `json:"tool_category" yaml:"tool_category"`
	OutputCommand string               `json:"output_command" yaml:"output_parameter"`
	SourceURL     string               `json:"source_url" yaml:"source_url"`
	DockerImage   string               `json:"docker_image" yaml:"docker_image"`
	Command       string               `json:"command" yaml:"command"`
	OutputType    string               `json:"output_type" yaml:"output_type"`
	Inputs        map[string]NodeInput `json:"inputs" yaml:"inputs"`
	LicenseInfo   struct {
		Name string `json:"name" yaml:"name"`
		Url  string `json:"url" yaml:"url"`
	} `json:"license_info" yaml:"license_info"`
	DocLink string `json:"doc_link" yaml:"doc_link"`
}

func (c *Client) ListPrivateTools(ctx context.Context) ([]Tool, error) {
	path := fmt.Sprintf("/library/tool/?public=False&vault=%s", c.vaultID)

	tools, err := GetPaginated[Tool](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools: %w", err)
	}

	return tools, nil
}

// GetPrivateToolByName gets a private tool by name
func (c *Client) GetPrivateToolByName(ctx context.Context, toolName string) (*Tool, error) {
	path := fmt.Sprintf("/library/tool/?public=False&vault=%s&name=%s", c.vaultID, toolName)

	tools, err := GetPaginated[Tool](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool: %w", err)
	}

	if len(tools) == 0 {
		return nil, fmt.Errorf("couldn't find tool %q", toolName)
	}

	return &tools[0], nil
}

// prepareToolImport prepares a tool import for creation by adding the vault ID, category ID, and output type, and setting the visible and type properties of the inputs
func (c *Client) prepareToolImport(ctx context.Context, tool *ToolImport) (*ToolImport, error) {
	tool.VaultInfo = &c.vaultID

	category, err := c.GetLibraryCategoryByName(ctx, tool.Category)
	if err != nil {
		return nil, fmt.Errorf("couldn't use the category %q: %w", tool.Category, err)
	}
	tool.CategoryID = &category.ID

	if tool.VaultInfo == nil {
		tool.VaultInfo = &c.vaultID
	}

	tool.OutputType = toolOutputTypes[tool.OutputType]
	for name := range tool.Inputs {
		if input, ok := tool.Inputs[name]; ok {
			if input.Visible == nil {
				input.Visible = &[]bool{false}[0]
			}
			input.Type = strings.ToUpper(tool.Inputs[name].Type)
			tool.Inputs[name] = input
		}
	}

	return tool, nil
}

// CreatePrivateTool creates a new private tool
func (c *Client) CreatePrivateTool(ctx context.Context, tool *ToolImport) (*ToolImport, error) {
	path := "/library/tool/"

	tool, err := c.prepareToolImport(ctx, tool)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare tool import: %w", err)
	}

	var createdTool ToolImport
	if err := c.Hive.doJSON(ctx, http.MethodPost, path, tool, &createdTool); err != nil {
		return nil, fmt.Errorf("failed to create private tool: %w", err)
	}

	return &createdTool, nil
}

// UpdatePrivateTool updates a private tool
func (c *Client) UpdatePrivateTool(ctx context.Context, tool *ToolImport, toolID uuid.UUID) (*ToolImport, error) {
	path := fmt.Sprintf("/library/tool/%s/", toolID)

	tool, err := c.prepareToolImport(ctx, tool)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare tool import: %w", err)
	}

	var updatedTool ToolImport
	if err := c.Hive.doJSON(ctx, http.MethodPatch, path, tool, &updatedTool); err != nil {
		return nil, fmt.Errorf("failed to update private tool: %w", err)
	}

	return &updatedTool, nil
}

// DeletePrivateTool deletes a private tool
func (c *Client) DeletePrivateTool(ctx context.Context, toolID uuid.UUID) error {
	path := fmt.Sprintf("/library/tool/%s/", toolID)

	if err := c.Hive.doJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete private tool: %w", err)
	}

	return nil
}
