package types

import (
	"time"

	"github.com/google/uuid"
)

type OutputsConfig struct {
	Outputs []string `yaml:"outputs"`
}

type SignedURL struct {
	Url        string `json:"url"`
	Size       int    `json:"size"`
	PrettySize string `json:"pretty_size"`
}

type SubJobOutputs struct {
	Next     string         `json:"next"`
	Previous string         `json:"previous"`
	Page     int            `json:"page"`
	Last     int            `json:"last"`
	Count    int            `json:"count"`
	Results  []SubJobOutput `json:"results"`
}

type SubJobOutput struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Size       int       `json:"size"`
	PrettySize string    `json:"pretty_size"`
	Format     string    `json:"format"`
	Path       string    `json:"path"`
	SignedURL  string    `json:"signed_url,omitempty"`
}

type WorkflowVersionData struct {
	Nodes          map[string]*Node          `json:"nodes"`
	Connections    []Connection              `json:"connections"`
	PrimitiveNodes map[string]*PrimitiveNode `json:"primitiveNodes"`
	Annotations    map[string]*Annotation    `json:"annotation,omitempty"`
}

type WorkflowVersionDetailed struct {
	ID           uuid.UUID           `json:"id"`
	Version      int                 `json:"version"`
	WorkflowInfo uuid.UUID           `json:"workflow_info"`
	Name         *string             `json:"name,omitempty"`
	Description  string              `json:"description"`
	Public       bool                `json:"public"`
	CreatedDate  time.Time           `json:"created_date"`
	RunCount     int                 `json:"run_count"`
	MaxMachines  Machines            `json:"max_machines"`
	Snapshot     bool                `json:"snapshot"`
	Data         WorkflowVersionData `json:"data"`
}
type WorkflowVersionStripped struct {
	ID           uuid.UUID           `json:"id"`
	WorkflowInfo uuid.UUID           `json:"workflow_info"`
	Name         *string             `json:"name,omitempty"`
	Description  string              `json:"description"`
	MaxMachines  Machines            `json:"max_machines"`
	Snapshot     bool                `json:"snapshot"`
	Data         WorkflowVersionData `json:"data"`
}

type Annotation struct {
	Content     string  `json:"content"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Name        string  `json:"name"`
	Coordinates struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"coordinates"`
}

type Connection struct {
	Source struct {
		ID string `json:"id"`
	} `json:"source"`
	Destination struct {
		ID string `json:"id"`
	} `json:"destination"`
}

type PrimitiveNode struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Label       string      `json:"label"`
	Value       interface{} `json:"value"`
	TypeName    string      `json:"type_name"`
	Coordinates struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"coordinates"`
	ParamName  *string `json:",omitempty"`
	UpdateFile *bool   `json:",omitempty"`
}

type Node struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Meta struct {
		Label       string `json:"label"`
		Coordinates struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"coordinates"`
	} `json:"meta"`
	Type   string                `json:"type"`
	Inputs map[string]*NodeInput `json:"inputs"`
	Script *struct {
		Args   []interface{} `json:"args"`
		Image  string        `json:"image"`
		Source string        `json:"source"`
	} `json:"script,omitempty"`
	Outputs   map[string]*NodeOutput `json:"outputs"`
	BeeType   string                 `json:"bee_type"`
	Container *struct {
		Args    []string `json:"args,omitempty"`
		Image   string   `json:"image"`
		Command []string `json:"command"`
	} `json:"container,omitempty"`
	OutputCommand   *string `json:"output_command,omitempty"`
	WorkerConnected *string `json:"workerConnected,omitempty"`
	Workflow        *string `json:"workflow,omitempty"`
}

type NodeInput struct {
	Type            string      `json:"type"`
	Order           int         `json:"order"`
	Value           interface{} `json:"value,omitempty"`
	Command         *string     `json:"command,omitempty"`
	Description     *string     `json:"description,omitempty"`
	WorkerConnected *bool       `json:"workerConnected,omitempty"`
	Multi           *bool       `json:"multi,omitempty"`
	Visible         *bool       `json:"visible,omitempty"`
}

type NodeOutput struct {
	Type          string  `json:"type"`
	Order         int     `json:"order"`
	ParameterName *string `json:"parameter_name,omitempty"`
	Visible       *bool   `json:"visible,omitempty"`
}
