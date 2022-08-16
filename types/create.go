package types

import (
	"github.com/google/uuid"
	"time"
)

type CreateSpaceRequest struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	VaultInfo   uuid.UUID `json:"vault_info"`
}

type CreateProjectRequest struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SpaceID     uuid.UUID `json:"space_info"`
}

type CreateWorkflowRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	SpaceID     uuid.UUID  `json:"space_info"`
	ProjectID   *uuid.UUID `json:"project_info,omitempty"`
}

type CreateWorkflowResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SpaceInfo    uuid.UUID `json:"space_info"`
	ProjectInfo  uuid.UUID `json:"project_info"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
	Playground   bool      `json:"playground"`
}

type CopyWorkflowRequest struct {
	SpaceID   uuid.UUID  `json:"space_info"`
	ProjectID *uuid.UUID `json:"project_info,omitempty"`
}

type CopyWorkflowResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SpaceInfo    uuid.UUID `json:"space_info"`
	ProjectInfo  uuid.UUID `json:"project_info"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
	Playground   bool      `json:"playground"`
}
