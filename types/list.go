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
	CreatedDate    time.Time  `json:"created_date"`
	ModifiedDate   time.Time  `json:"modified_date"`
}

type Project struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	SpaceInfo     uuid.UUID  `json:"space_info"`
	SpaceName     string     `json:"space_name"`
	WorkflowCount int        `json:"workflow_count"`
	CreatedDate   time.Time  `json:"created_date"`
	ModifiedDate  time.Time  `json:"modified_date"`
	Author        string     `json:"author"`
	Workflows     []Workflow `json:"workflows,omitempty"`
}

type Workflows struct {
	Next     string     `json:"next"`
	Previous string     `json:"previous"`
	Page     int        `json:"page"`
	Last     int        `json:"last"`
	Count    int        `json:"count"`
	Results  []Workflow `json:"results"`
}

type Workflow struct {
	ID               uuid.UUID     `json:"id,omitempty"`
	Name             string        `json:"name,omitempty"`
	Description      string        `json:"description,omitempty"`
	SpaceInfo        *uuid.UUID    `json:"space_info,omitempty"`
	SpaceName        string        `json:"space_name,omitempty"`
	ProjectInfo      *uuid.UUID    `json:"project_info,omitempty"`
	ProjectName      string        `json:"project_name,omitempty"`
	ModifiedDate     *time.Time    `json:"modified_date,omitempty"`
	CreatedDate      time.Time     `json:"created_date,omitempty"`
	ScheduleInfo     *ScheduleInfo `json:"schedule_info,omitempty"`
	WorkflowCategory string        `json:"workflow_category,omitempty"`
	Author           string        `json:"author,omitempty"`
	Executing        bool          `json:"executing,omitempty"`
}

type ScheduleInfo struct {
	ID           string     `json:"id,omitempty"`
	Vault        string     `json:"vault,omitempty"`
	Date         *time.Time `json:"date,omitempty"`
	Workflow     string     `json:"workflow,omitempty"`
	RepeatPeriod int        `json:"repeat_period,omitempty"`
	Machines     *Machines  `json:"machines,omitempty"`
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
	ID                  *uuid.UUID `json:"id,omitempty"`
	Name                string     `json:"name,omitempty"`
	Status              string     `json:"status,omitempty"`
	Machines            Machines   `json:"machines,omitempty"`
	WorkflowVersionInfo *uuid.UUID `json:"workflow_version_info,omitempty"`
	WorkflowInfo        *uuid.UUID `json:"workflow_info,omitempty"`
	WorkflowName        string     `json:"workflow_name,omitempty"`
	SpaceInfo           *uuid.UUID `json:"space_info,omitempty"`
	SpaceName           string     `json:"space_name,omitempty"`
	ProjectInfo         *uuid.UUID `json:"project_info,omitempty"`
	ProjectName         string     `json:"project_name,omitempty"`
	CreationType        string     `json:"creation_type,omitempty"`
	CreatedDate         time.Time  `json:"created_date,omitempty"`
	StartedDate         time.Time  `json:"started_date,omitempty"`
	CompletedDate       time.Time  `json:"completed_date,omitempty"`
	Finished            bool       `json:"finished,omitempty"`
	Author              string     `json:"author,omitempty"`
	Fleet               *uuid.UUID `json:"fleet,omitempty"`
	IPAddresses         []string   `json:"ip_addresses,omitempty"`
}

type Machines struct {
	Small      *int `json:"small,omitempty"`
	Medium     *int `json:"medium,omitempty"`
	Large      *int `json:"large,omitempty"`
	Default    *int `json:"default,omitempty"`
	SelfHosted *int `json:"self_hosted,omitempty"`
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
	Status        string                 `json:"status"`
	Name          string                 `json:"name"`
	OutputsStatus string                 `json:"outputs_status"`
	Finished      bool                   `json:"finished"`
	StartedDate   time.Time              `json:"started_at"`
	FinishedDate  time.Time              `json:"finished_at"`
	Params        map[string]interface{} `json:"params"`
	Message       string                 `json:"message"`
	TaskGroup     bool                   `json:"task_group"`
	TaskIndex     int                    `json:"task_index"`
	Label         string
	Children      []SubJob
	TaskCount     int
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
	ID               uuid.UUID            `json:"id"`
	Name             string               `json:"name"`
	Description      string               `json:"description"`
	VaultInfo        uuid.UUID            `json:"vault_info"`
	Author           string               `json:"author"`
	AuthorInfo       int                  `json:"author_info"`
	ToolCategory     string               `json:"tool_category"`
	ToolCategoryName string               `json:"tool_category_name"`
	Type             string               `json:"type"`
	Inputs           map[string]ToolInput `json:"inputs"`
	Container        *struct {
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
	DocLink string `json:"doc_link"`
}

type ToolInput struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Order       int    `json:"order"`
	Visible     bool   `json:"visible"`
}

type Categories struct {
    Next     string     `json:"next"`
    Previous string     `json:"previous"`
    Page     int        `json:"page"`
    Last     int        `json:"last"`
    Count    int        `json:"count"`
    Results  []Category `json:"results"`
}

type Category struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	WorkflowCount int       `json:"workflow_count"`
	ToolCount     int       `json:"tool_count"`
}
