package output

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/trickest/trickest-cli/pkg/config"
	"gopkg.in/yaml.v3"
)

// Config holds the configuration for the output command
type Config struct {
	Token   string
	BaseURL string

	ConfigFile string

	Nodes string
	Files string

	AllRuns      bool
	NumberOfRuns int
	RunID        string
	RunSpec      config.RunSpec

	OutputDir string
}

// OutputsConfig is the yaml configuration file format for the output command
type OutputsConfig struct {
	Outputs []string `yaml:"outputs"`
}

// readNodesFromFile reads the nodes from the config file
func (c *Config) readNodesFromFile() ([]string, error) {
	var nodes []string

	file, err := os.Open(c.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("couldn't open config file to read outputs: %w", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("couldn't read outputs config: %w", err)
	}

	var conf OutputsConfig
	err = yaml.Unmarshal(bytes, &conf)
	if err != nil {
		return nil, fmt.Errorf("couldn't unmarshal outputs config: %w", err)
	}

	for _, node := range conf.Outputs {
		nodes = append(nodes, strings.ReplaceAll(node, "/", "-"))
	}

	return nodes, nil
}

// GetNodes returns the nodes to download from the command line flag, config file, or URL
func (c *Config) GetNodes() []string {
	var nodes []string

	if c.Nodes != "" {
		for _, node := range strings.Split(c.Nodes, ",") {
			node = strings.TrimSpace(node)
			if node != "" {
				nodes = append(nodes, strings.ReplaceAll(node, "/", "-"))
			}
		}
		return nodes
	}

	if c.ConfigFile != "" {
		if fileNodes, err := c.readNodesFromFile(); err == nil {
			nodes = append(nodes, fileNodes...)
			return nodes
		}
	}

	if c.RunSpec.URL != "" {
		u, err := url.Parse(c.RunSpec.URL)
		if err == nil {
			queryParams, err := url.ParseQuery(u.RawQuery)
			if err == nil {
				if nodeParams, found := queryParams["node"]; found && len(nodeParams) == 1 {
					node := nodeParams[0]
					if node != "" {
						nodes = append(nodes, node)
					}
				}
			}
		}
	}

	return nodes
}

// GetFiles returns the files to download from the command line flag
func (c *Config) GetFiles() []string {
	if c.Files != "" {
		return strings.Split(c.Files, ",")
	}
	return []string{}
}

// GetOutputPath returns the output directory path, either from the config or constructed from space/project/workflow
func (c *Config) GetOutputPath() string {
	if c.OutputDir != "" {
		return c.OutputDir
	}
	return c.formatPath()
}

// formatPath formats the path for the output command based on the space, project, and workflow names if they are provided
func (c *Config) formatPath() string {
	path := strings.Trim(c.RunSpec.SpaceName, "/")
	if c.RunSpec.ProjectName != "" {
		path += "/" + strings.Trim(c.RunSpec.ProjectName, "/")
	}
	if c.RunSpec.WorkflowName != "" {
		path += "/" + strings.Trim(c.RunSpec.WorkflowName, "/")
	}
	return path
}
