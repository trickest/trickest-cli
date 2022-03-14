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
	HasParent    bool
	Children     []*TreeNode
	Status       string
	Message      string
	OutputStatus string
	Duration     time.Duration
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
