package types

import (
	"time"

	"github.com/google/uuid"
)

type WorkflowVersions struct {
	Next     string            `json:"next"`
	Previous string            `json:"previous"`
	Page     int               `json:"page"`
	Last     int               `json:"last"`
	Count    int               `json:"count"`
	Results  []WorkflowVersion `json:"results"`
}

type WorkflowVersion struct {
	ID           uuid.UUID `json:"id"`
	CreatedDate  time.Time `json:"created_date"`
	Version      int       `json:"version"`
	WorkflowInfo uuid.UUID `json:"workflow_info"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Public       bool      `json:"public"`
	RunCount     int       `json:"run_count"`
	Snapshot     bool      `json:"snapshot"`
}

type TreeNode struct {
	Name         string
	Label        string
	Inputs       *map[string]*NodeInput
	Printed      bool
	Parents      []*TreeNode
	Children     []*TreeNode
	Status       string
	Message      string
	OutputStatus string
	Duration     time.Duration
	Height       int
}

type CreateRun struct {
	Machines     Machines   `json:"machines"`
	VersionID    uuid.UUID  `json:"workflow_version_info"`
	Vault        uuid.UUID  `json:"vault"`
	Fleet        *uuid.UUID `json:"fleet,omitempty"`
	UseStaticIPs bool       `json:"use_static_ips"`
}

type CreateRunResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Machines  Machines  `json:"machines"`
	VersionID uuid.UUID `json:"workflow_version_info"`
	HiveInfo  uuid.UUID `json:"hive_info"`
}

type WorkflowYAML struct {
	Name     string             `yaml:"name"`
	Category *string            `yaml:"category,omitempty"`
	Steps    []WorkflowYAMLNode `yaml:"steps"`
}

type WorkflowYAMLNode struct {
	Name    string      `yaml:"name"`
	ID      string      `yaml:"id"`
	Script  *string     `yaml:"script,omitempty"`
	Machine string      `yaml:"machine"`
	Inputs  interface{} `yaml:"inputs"`
}

type Scripts struct {
	Next     string   `json:"next"`
	Previous string   `json:"previous"`
	Page     int      `json:"page"`
	Last     int      `json:"last"`
	Count    int      `json:"count"`
	Results  []Script `json:"results"`
}

type Script struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	VaultInfo   uuid.UUID `json:"vault_info"`
	Author      string    `json:"author"`
	AuthorInfo  int       `json:"author_info"`
	Type        string    `json:"type"`
	Inputs      struct {
		File *struct {
			Type  string `json:"type"`
			Multi bool   `json:"multi"`
		} `json:"file,omitempty"`
		Folder *struct {
			Type  string `json:"type"`
			Multi bool   `json:"multi"`
		} `json:"folder,omitempty"`
	} `json:"inputs"`
	Outputs struct {
		File   NodeOutput `json:"file,omitempty"`
		Folder NodeOutput `json:"folder,omitempty"`
	} `json:"outputs"`
	Script struct {
		Args   []interface{} `json:"args"`
		Image  string        `json:"image"`
		Source string        `json:"source"`
	} `json:"script"`
}

type SplitterResponse struct {
	Next     string     `json:"next"`
	Previous string     `json:"previous"`
	Page     int        `json:"page"`
	Last     int        `json:"last"`
	Count    int        `json:"count"`
	Results  []Splitter `json:"results"`
}

type Splitter struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	VaultInfo   uuid.UUID `json:"vault_info"`
	Author      string    `json:"author"`
	AuthorInfo  int       `json:"author_info"`
	Type        string    `json:"type"`
	Inputs      struct {
		Multiple struct {
			Type  string `json:"type"`
			Multi bool   `json:"multi"`
		} `json:"multiple"`
	} `json:"inputs"`
	Outputs struct {
		Output struct {
			Type  string `json:"type"`
			Order *int   `json:"order,omitempty"`
		} `json:"output"`
	} `json:"outputs"`
}

type FilesResponse struct {
	Next     string         `json:"next"`
	Previous string         `json:"previous"`
	Page     int            `json:"page"`
	Last     int            `json:"last"`
	Count    int            `json:"count"`
	Results  []TrickestFile `json:"results"`
}

type TrickestFile struct {
	Id           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	VaultInfo    uuid.UUID `json:"vault_info"`
	Size         int       `json:"size"`
	ModifiedDate time.Time `json:"modified_date"`
}
