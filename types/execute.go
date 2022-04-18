package types

import (
	"time"
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
	ID           string    `json:"id"`
	CreatedDate  time.Time `json:"created_date"`
	Version      int       `json:"version"`
	WorkflowInfo string    `json:"workflow_info"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Public       bool      `json:"public"`
	RunCount     int       `json:"run_count"`
}

type TreeNode struct {
	NodeName     string
	Label        string
	Inputs       *map[string]*NodeInput
	Printed      bool
	Parent       *TreeNode
	Children     []*TreeNode
	Status       string
	Message      string
	OutputStatus string
	Duration     time.Duration
	Height       int
}

type CreateRun struct {
	Bees      Bees   `json:"bees"`
	VersionID string `json:"workflow_version_info"`
	HiveInfo  string `json:"hive_info"`
}

type CreateRunResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Bees      Bees   `json:"bees"`
	VersionID string `json:"workflow_version_info"`
	HiveInfo  string `json:"hive_info"`
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	VaultInfo   string `json:"vault_info"`
	Author      string `json:"author"`
	AuthorInfo  int    `json:"author_info"`
	Type        string `json:"type"`
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
		File *struct {
			Type string `json:"type"`
		} `json:"file,omitempty"`
		Folder *struct {
			Type string `json:"type"`
		} `json:"folder,omitempty"`
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	VaultInfo   string `json:"vault_info"`
	Author      string `json:"author"`
	AuthorInfo  int    `json:"author_info"`
	Type        string `json:"type"`
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
