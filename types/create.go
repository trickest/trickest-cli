package types

import "time"

type CreateSpaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	VaultInfo   string `json:"vault_info"`
}

type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SpaceID     string `json:"space_info"`
}

type CreateWorkflowRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SpaceID     string `json:"space_info"`
	ProjectID   string `json:"project_info"`
}

type CreateWorkflowResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SpaceInfo    string    `json:"space_info"`
	ProjectInfo  string    `json:"project_info"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
	Playground   bool      `json:"playground"`
}

type CopyWorkflowRequest struct {
	SpaceID   string `json:"space_info"`
	ProjectID string `json:"project_info"`
}

type CopyWorkflowResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SpaceInfo    string    `json:"space_info"`
	ProjectInfo  string    `json:"project_info"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
	Playground   bool      `json:"playground"`
}
