package types

import "time"

type Spaces struct {
	Next     string  `json:"next"`
	Previous string  `json:"previous"`
	Page     int     `json:"page"`
	Last     int     `json:"last"`
	Count    int     `json:"count"`
	Results  []Space `json:"results"`
}

type Space struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	VaultInfo    string    `json:"vault_info"`
	Metadata     string    `json:"metadata"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
	Playground   bool      `json:"playground"`
}

type SpaceDetailed struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	VaultInfo      string     `json:"vault_info"`
	Playground     bool       `json:"playground"`
	Projects       []Project  `json:"projects"`
	ProjectsCount  int        `json:"projects_count"`
	Workflows      []Workflow `json:"workflows"`
	WorkflowsCount int        `json:"workflows_count"`
	Metadata       string     `json:"metadata"`
	CreatedDate    time.Time  `json:"created_date"`
	ModifiedDate   time.Time  `json:"modified_date"`
}

type Project struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	SpaceInfo     string    `json:"space_info"`
	SpaceName     string    `json:"space_name"`
	Metadata      string    `json:"metadata"`
	WorkflowCount int       `json:"workflow_count"`
	CreatedDate   time.Time `json:"created_date"`
	ModifiedDate  time.Time `json:"modified_date"`
	Workflows     []WorkflowListResponse
}

type Workflows struct {
	Next     string                 `json:"next"`
	Previous string                 `json:"previous"`
	Page     int                    `json:"page"`
	Last     int                    `json:"last"`
	Count    int                    `json:"count"`
	Results  []WorkflowListResponse `json:"results"`
}

type WorkflowListResponse struct {
	ID               string    `json:"id"`
	CreatedDate      time.Time `json:"created_date"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	SpaceName        string    `json:"space_name"`
	RunCount         int       `json:"run_count"`
	WorkflowCategory string    `json:"workflow_category"`
}

type Workflow struct {
	ID               string    `json:"id"`
	CreatedDate      time.Time `json:"created_date"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	SpaceName        string    `json:"space_name"`
	RunCount         int       `json:"run_count"`
	WorkflowCategory *struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"workflow_category,omitempty"`
	WorkflowTags []string    `json:"workflow_tags,omitempty"`
	Public       *bool       `json:"public,omitempty"`
	Author       string      `json:"author,omitempty"`
	AuthorInfo   int         `json:"author_info,omitempty"`
	Parameters   []Parameter `json:"parameters,omitempty"`
	ModifiedDate *time.Time  `json:"modified_date,omitempty"`
	SpaceInfo    string      `json:"space_info,omitempty"`
	ProjectName  string      `json:"project_name,omitempty"`
	ProjectInfo  string      `json:"project_info,omitempty"`
	VersionCount *int        `json:"version_count,omitempty"`
}
type Parameter struct {
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}
