package types

import "time"

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
	ID         string `json:"id"`
	FileName   string `json:"file_name"`
	Size       int    `json:"size"`
	PrettySize string `json:"pretty_size"`
	Format     string `json:"format"`
	Path       string `json:"path"`
	SignedURL  string `json:"signed_url,omitempty"`
}

type WorkflowVersionDetailed struct {
	ID           string    `json:"id"`
	Version      int       `json:"version"`
	WorkflowInfo string    `json:"workflow_info"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Public       bool      `json:"public"`
	CreatedDate  time.Time `json:"created_date"`
	RunCount     int       `json:"run_count"`
	MaxMachines  Bees      `json:"max_machines"`
	Data         struct {
		Nodes       map[string]Node `json:"nodes"`
		Connections []struct {
			Source struct {
				ID string `json:"id"`
			} `json:"source"`
			Destination struct {
				ID string `json:"id"`
			} `json:"destination"`
		} `json:"connections"`
		PrimitiveNodes map[string]struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Label       string `json:"label"`
			Value       string `json:"value"`
			TypeName    string `json:"type_name"`
			Coordinates struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
			}
		}
	}
}

type Node struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Meta struct {
		Label       string `json:"label"`
		Coordinates struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}
	} `json:"meta"`
	Type    string               `json:"type"`
	Inputs  map[string]NodeInput `json:"inputs"`
	Outputs struct {
		Folder *struct {
			Type  string `json:"type"`
			Order int    `json:"order"`
		} `json:"folder"`
		File *struct {
			Type  string `json:"type"`
			Order int    `json:"order"`
		} `json:"file"`
	} `json:"outputs"`
	BeeType   string `json:"bee_type"`
	Container *struct {
		Image   string   `json:"image"`
		Command []string `json:"command"`
	}
	OutputCommand   string `json:"output_command"`
	WorkerConnected string `json:"workerConnected"`
}

type NodeInput struct {
	Type            string `json:"type"`
	Order           int    `json:"order"`
	Value           string `json:"value,omitempty"`
	Command         string `json:"command,omitempty"`
	Description     string `json:"description,omitempty"`
	WorkerConnected *bool  `json:"workerConnected,omitempty"`
	Multi           *bool  `json:"multi,omitempty"`
}
