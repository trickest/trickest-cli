package trickest

import (
	"time"

	"github.com/google/uuid"
)

// Project represents a project
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
