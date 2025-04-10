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
		Input  map[string]any `yaml:"input"`  // To match the input flag
		Inputs map[string]any `yaml:"inputs"` // For backward compatibility

		Output  any `yaml:"output"`  // To match the output flag
		Outputs any `yaml:"outputs"` // For backward compatibility

		Machines     int    `yaml:"machines"`
		UseStaticIPs bool   `yaml:"use-static-ips"`
		Fleet        string `yaml:"fleet"`
	}

	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal run config: %w", err)
	}

	inputs := rawConfig.Inputs
	if inputs == nil {
		inputs = rawConfig.Input
	}

	nodeInputs, primitiveNodeInputs, err := ParseInputs(inputs)
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

	rawOutputs := rawConfig.Outputs
	if rawOutputs == nil {
		rawOutputs = rawConfig.Output
	}

	// Handle a single output or a list of outputs
	if rawOutputs != nil {
		switch v := rawOutputs.(type) {
		case string:
			config.Outputs = append(config.Outputs, v)
		case []any:
			for _, item := range v {
				config.Outputs = append(config.Outputs, fmt.Sprintf("%v", item))
			}
		default:
			config.Outputs = append(config.Outputs, fmt.Sprintf("%v", v))
		}
	}

	return config, nil
}
