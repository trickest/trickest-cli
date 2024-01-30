package types

import "github.com/google/uuid"

type ScriptImportRequest struct {
	ID          *uuid.UUID `json:"id,omitempty"`
	VaultInfo   uuid.UUID  `json:"vault_info" yaml:"vault_info"`
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description" yaml:"description"`
	DockerImage string     `json:"docker_image" yaml:"docker_image"`
	Script      string     `json:"script" yaml:"script"`
	Command     string     `json:"command" yaml:"command"`
}
