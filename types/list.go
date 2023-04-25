package types

import (
	"time"

	"github.com/google/uuid"
)

type Spaces struct {
	Next     string  `json:"next"`
	Previous string  `json:"previous"`
	Page     int     `json:"page"`
	Last     int     `json:"last"`
	Count    int     `json:"count"`
	Results  []Space `json:"results"`
}

type Space struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	VaultInfo    uuid.UUID `json:"vault_info"`
	Metadata     string    `json:"metadata"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
	Playground   bool      `json:"playground"`
}

type SpaceDetailed struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	VaultInfo      uuid.UUID  `json:"vault_info"`
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
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	SpaceInfo     uuid.UUID `json:"space_info"`
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
	ID               uuid.UUID `json:"id"`
	CreatedDate      time.Time `json:"created_date"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	SpaceName        string    `json:"space_name"`
	SpaceInfo        uuid.UUID `json:"space_info"`
	ProjectInfo      uuid.UUID `json:"project_info"`
	RunCount         int       `json:"run_count"`
	WorkflowCategory string    `json:"workflow_category"`
	IsScheduled      bool      `json:"is_scheduled"`
	Author           string    `json:"author"`
	AuthorInfo       int       `json:"author_info"`
	LatestVersion    uuid.UUID `json:"latest_version"`
}

type Workflow struct {
	ID               uuid.UUID `json:"id"`
	CreatedDate      time.Time `json:"created_date"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	SpaceName        string    `json:"space_name"`
	RunCount         int       `json:"run_count"`
	LatestVersion    uuid.UUID `json:"latest_version"`
	WorkflowCategory *struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
	} `json:"workflow_category,omitempty"`
	WorkflowTags []string    `json:"workflow_tags,omitempty"`
	Public       *bool       `json:"public,omitempty"`
	Author       string      `json:"author,omitempty"`
	AuthorInfo   int         `json:"author_info,omitempty"`
	Parameters   []Parameter `json:"parameters,omitempty"`
	ModifiedDate *time.Time  `json:"modified_date,omitempty"`
	SpaceInfo    uuid.UUID   `json:"space_info,omitempty"`
	ProjectName  string      `json:"project_name,omitempty"`
	ProjectInfo  uuid.UUID   `json:"project_info,omitempty"`
	VersionCount *int        `json:"version_count,omitempty"`
	IsScheduled  bool        `json:"is_scheduled"`
	Executing    bool        `json:"executing"`
}
type Parameter struct {
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

type Runs struct {
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Page     int    `json:"page"`
	Last     int    `json:"last"`
	Count    int    `json:"count"`
	Results  []Run  `json:"results"`
}

const (
	RunCreationManual    = "MANUAL"
	RunCreationScheduled = "SCHEDULED"
)

type Run struct {
	ID                  uuid.UUID `json:"id"`
	Name                string    `json:"name"`
	Status              string    `json:"status"`
	UserInfo            int       `json:"user_info"`
	SpaceInfo           uuid.UUID `json:"space_info"`
	SpaceName           string    `json:"space_name"`
	ProjectInfo         uuid.UUID `json:"project_info"`
	ProjectName         string    `json:"project_name"`
	WorkflowInfo        uuid.UUID `json:"workflow_info"`
	WorkflowName        string    `json:"workflow_name"`
	WorkflowVersionName string    `json:"workflow_version_name"`
	WorkflowVersionInfo uuid.UUID `json:"workflow_version_info"`
	HiveInfo            uuid.UUID `json:"hive_info"`
	StartedDate         time.Time `json:"started_date"`
	CompletedDate       time.Time `json:"completed_date"`
	Parallelism         int       `json:"parallelism"`
	Machines            Machines  `json:"machines"`
	CreatedDate         time.Time `json:"created_date"`
	ModifiedDate        time.Time `json:"modified_date"`
	Finished            bool      `json:"finished"`
	CreationType        string    `json:"creation_type"`
}

type Machines struct {
	Small  *int `json:"small,omitempty"`
	Medium *int `json:"medium,omitempty"`
	Large  *int `json:"large,omitempty"`
}

type SubJobs struct {
	Next     string   `json:"next"`
	Previous string   `json:"previous"`
	Page     int      `json:"page"`
	Last     int      `json:"last"`
	Count    int      `json:"count"`
	Results  []SubJob `json:"results"`
}

type SubJob struct {
	ID            uuid.UUID              `json:"id"`
	Name          string                 `json:"name"`
	Status        string                 `json:"status"`
	StartedDate   time.Time              `json:"started_at"`
	FinishedDate  time.Time              `json:"finished_at"`
	Podname       string                 `json:"podname"`
	Params        map[string]interface{} `json:"params"`
	Message       string                 `json:"message"`
	TaskIndex     string                 `json:"task_index"`
	TaskCount     int                    `json:"task_count"`
	OutputsStatus string                 `json:"outputs_status"`
	Finished      bool                   `json:"finished"`
	TaskGroup     bool                   `json:"task_group"`
	Children      []SubJob
	Label         string
}

type Tools struct {
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Page     int    `json:"page"`
	Last     int    `json:"last"`
	Count    int    `json:"count"`
	Results  []Tool `json:"results"`
}

type Tool struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	VaultInfo        uuid.UUID `json:"vault_info"`
	Author           string    `json:"author"`
	AuthorInfo       int       `json:"author_info"`
	ToolCategory     string    `json:"tool_category"`
	ToolCategoryName string    `json:"tool_category_name"`
	ToolCategoryObj  struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
	} `json:"tool_category_obj"`
	Type      string               `json:"type"`
	Inputs    map[string]ToolInput `json:"inputs"`
	Container *struct {
		Args    []string `json:"args,omitempty"`
		Image   string   `json:"image"`
		Command []string `json:"command"`
	} `json:"container,omitempty"`
	Outputs struct {
		Folder *struct {
			Type  string `json:"type"`
			Order int    `json:"order"`
		} `json:"folder,omitempty"`
		File *struct {
			Type  string `json:"type"`
			Order int    `json:"order"`
		} `json:"file,omitempty"`
	} `json:"outputs"`
	SourceURL     string    `json:"source_url"`
	CreatedDate   time.Time `json:"created_date"`
	ModifiedDate  time.Time `json:"modified_date"`
	OutputCommand string    `json:"output_command"`
	LicenseInfo   struct {
		Name string `json:"name"`
		Url  string `json:"url"`
	} `json:"license_info"`
}

type ToolInput struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Order       int    `json:"order"`
}
