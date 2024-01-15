package types

import "github.com/google/uuid"

type ToolImportRequest struct {
	VaultInfo     uuid.UUID            `json:"vault_info" yaml:"vault_info"`
	Name          string               `json:"name" yaml:"name"`
	Description   string               `json:"description" yaml:"description"`
	Category      string               `json:"tool_category_name" yaml:"category"`
	CategoryID    uuid.UUID            `json:"tool_category" yaml:"tool_category"`
	OutputCommand string               `json:"output_command" yaml:"output_parameter"`
	SourceURL     string               `json:"source_url" yaml:"source_url"`
	DockerImage   string               `json:"docker_image" yaml:"docker_image"`
	Command       string               `json:"command" yaml:"command"`
	OutputType    string               `json:"output_type" yaml:"output_type"`
	Inputs        map[string]ToolInput `json:"inputs" yaml:"inputs"`
	LicenseInfo   struct {
		Name string `json:"name" yaml:"name"`
		Url  string `json:"url" yaml:"url"`
	} `json:"license_info" yaml:"license_info"`
	DocLink string `json:"doc_link" yaml:"doc_link"`
}
