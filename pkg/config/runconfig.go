package config

import (
	"fmt"
	"os"

	"github.com/trickest/trickest-cli/pkg/workflowbuilder"
	"gopkg.in/yaml.v3"
)

// RunConfig represents the configuration for a run
type RunConfig struct {
	NodeInputs          []workflowbuilder.NodeInput          `yaml:"-"`
	PrimitiveNodeInputs []workflowbuilder.PrimitiveNodeInput `yaml:"-"`
	Outputs             []string                             `yaml:"outputs"`
	Machines            int                                  `yaml:"machines"`
	UseStaticIPs        bool                                 `yaml:"use-static-ips"`
	Fleet               string                               `yaml:"fleet"`
}

func ParseConfigFile(path string) (*RunConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read run config file: %w", err)
	}
	return parseRunConfig(data)
}

func parseRunConfig(data []byte) (*RunConfig, error) {
	var rawConfig struct {
		Inputs       map[string]any `yaml:"inputs"`
		Outputs      any            `yaml:"outputs"`
		Machines     int            `yaml:"machines"`
		UseStaticIPs bool           `yaml:"use-static-ips"`
		Fleet        string         `yaml:"fleet"`
	}

	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal run config: %w", err)
	}

	nodeInputs, primitiveNodeInputs, err := ParseInputs(rawConfig.Inputs)
	if err != nil {
		return nil, err
	}

	config := &RunConfig{
		NodeInputs:          nodeInputs,
		PrimitiveNodeInputs: primitiveNodeInputs,
		Machines:            rawConfig.Machines,
		UseStaticIPs:        rawConfig.UseStaticIPs,
		Fleet:               rawConfig.Fleet,
	}

	// Handle a single output or a list of outputs
	outputs := []string{}
	if rawConfig.Outputs != nil {
		switch v := rawConfig.Outputs.(type) {
		case string:
			outputs = append(outputs, v)
		case []any:
			for _, item := range v {
				outputs = append(outputs, fmt.Sprintf("%v", item))
			}
		default:
			outputs = append(outputs, fmt.Sprintf("%v", v))
		}
	}
	config.Outputs = outputs

	return config, nil
}
