package types

import "github.com/google/uuid"

type ScriptImportRequest struct {
	ID          *uuid.UUID `json:"id,omitempty"`
	VaultInfo   uuid.UUID  `json:"vault_info" yaml:"vault_info"`
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description" yaml:"description"`
	ScriptType  string     `json:"script_type" yaml:"script_type"`
	Script      string     `json:"script" yaml:"script"`
}
