package trickest

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

type Script struct {
	ID          *uuid.UUID `json:"id,omitempty"`
	Name        string     `json:"name,omitempty"`
	Description string     `json:"description,omitempty"`
	VaultInfo   *uuid.UUID `json:"vault_info,omitempty"`
	Author      string     `json:"author,omitempty"`
	AuthorInfo  int        `json:"author_info,omitempty"`
	Type        string     `json:"type,omitempty"`
	Inputs      struct {
		File   NodeInput `json:"file,omitempty"`
		Folder NodeInput `json:"folder,omitempty"`
	} `json:"inputs,omitempty"`
	Outputs struct {
		File   NodeOutput `json:"file,omitempty"`
		Folder NodeOutput `json:"folder,omitempty"`
	} `json:"outputs,omitempty"`
	Script struct {
		Args   []any  `json:"args,omitempty"`
		Image  string `json:"image,omitempty"`
		Source string `json:"source,omitempty"`
	} `json:"script,omitempty"`
	Command    string `json:"command,omitempty"`
	ScriptType string `json:"script_type,omitempty"`
}

type ScriptImport struct {
	ID          *uuid.UUID `json:"id,omitempty"`
	VaultInfo   *uuid.UUID `json:"vault_info" yaml:"vault_info"`
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description" yaml:"description"`
	ScriptType  string     `json:"script_type" yaml:"script_type"`
	Script      string     `json:"script" yaml:"script"`
}

// ListPrivateScripts lists all private scripts
func (c *Client) ListPrivateScripts(ctx context.Context) ([]Script, error) {
	path := fmt.Sprintf("/library/script/?public=False&vault=%s", c.vaultID)

	scripts, err := GetPaginated[Script](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get scripts: %w", err)
	}

	return scripts, nil
}

// GetPrivateScriptByName gets a private script by name
func (c *Client) GetPrivateScriptByName(ctx context.Context, scriptName string) (*Script, error) {
	path := fmt.Sprintf("/library/script/?public=False&vault=%s&name=%s", c.vaultID, scriptName)

	scripts, err := GetPaginated[Script](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get script: %w", err)
	}

	if len(scripts) == 0 {
		return nil, fmt.Errorf("couldn't find script %q", scriptName)
	}

	return &scripts[0], nil
}

// CreatePrivateScript creates a new private script
func (c *Client) CreatePrivateScript(ctx context.Context, script *ScriptImport) (*ScriptImport, error) {
	path := "/script/"

	script.VaultInfo = &c.vaultID

	var createdScript ScriptImport
	if err := c.Hive.doJSON(ctx, http.MethodPost, path, script, &createdScript); err != nil {
		return nil, fmt.Errorf("failed to create private script: %w", err)
	}

	return &createdScript, nil
}

// UpdatePrivateScript updates a private script
func (c *Client) UpdatePrivateScript(ctx context.Context, script *ScriptImport, scriptID uuid.UUID) (*ScriptImport, error) {
	path := fmt.Sprintf("/script/%s/", scriptID)

	var updatedScript ScriptImport
	if err := c.Hive.doJSON(ctx, http.MethodPatch, path, script, &updatedScript); err != nil {
		return nil, fmt.Errorf("failed to update private script: %w", err)
	}

	return &updatedScript, nil
}

// DeletePrivateScript deletes a private script
func (c *Client) DeletePrivateScript(ctx context.Context, scriptID uuid.UUID) error {
	path := fmt.Sprintf("/script/%s/", scriptID)

	if err := c.Hive.doJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete private script: %w", err)
	}

	return nil
}
