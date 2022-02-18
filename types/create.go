package types

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
