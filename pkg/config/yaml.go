package config

import (
	"fmt"
	"os"
	"strings"

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

	config := &RunConfig{
		NodeInputs:          make([]workflowbuilder.NodeInput, 0),
		PrimitiveNodeInputs: make([]workflowbuilder.PrimitiveNodeInput, 0),
		Machines:            rawConfig.Machines,
		UseStaticIPs:        rawConfig.UseStaticIPs,
		Fleet:               rawConfig.Fleet,
	}

	for key, value := range rawConfig.Inputs {
		if strings.Contains(key, ".") {
			// node-ref.param-name input
			parts := strings.Split(key, ".")
			if len(parts) != 2 {
				continue
			}
			nodeRef := parts[0]
			paramName := parts[1]

			// Handle a list of values or a single value
			switch v := value.(type) {
			case []any:
				config.NodeInputs = append(config.NodeInputs, workflowbuilder.NodeInput{
					NodeID:      nodeRef,
					ParamValues: map[string][]any{paramName: v},
				})
			default:
				config.NodeInputs = append(config.NodeInputs, workflowbuilder.NodeInput{
					NodeID:      nodeRef,
					ParamValues: map[string][]any{paramName: {v}},
				})
			}
		} else {
			// primitive node reference
			switch v := value.(type) {
			case []any:
				return nil, fmt.Errorf("invalid input for node %q: got an array of values %v. For primitive input nodes, use a single value '%s: <value>'. For tool/module/script input nodes, use the node-reference format '%s.param-name: <values>", key, v, key, key)
			default:
				config.PrimitiveNodeInputs = append(config.PrimitiveNodeInputs, workflowbuilder.PrimitiveNodeInput{
					PrimitiveNodeID: key,
					Value:           value,
				})
			}
		}
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
