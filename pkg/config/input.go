package config

import (
	"fmt"
	"strings"

	"github.com/trickest/trickest-cli/pkg/workflowbuilder"
)

func ParseInputs(inputs map[string]any) ([]workflowbuilder.NodeInput, []workflowbuilder.PrimitiveNodeInput, error) {
	nodeInputs := make([]workflowbuilder.NodeInput, 0)
	primitiveNodeInputs := make([]workflowbuilder.PrimitiveNodeInput, 0)

	for key, value := range inputs {
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
				nodeInputs = append(nodeInputs, workflowbuilder.NodeInput{
					NodeID:      nodeRef,
					ParamValues: map[string][]any{paramName: v},
				})
			default:
				nodeInputs = append(nodeInputs, workflowbuilder.NodeInput{
					NodeID:      nodeRef,
					ParamValues: map[string][]any{paramName: {v}},
				})
			}
		} else {
			// primitive node reference
			switch v := value.(type) {
			case []any:
				return nil, nil, fmt.Errorf("invalid input for node %q: got an array of values %v. For primitive input nodes, use a single value '%s: <value>'. For tool/module/script input nodes, use the node-reference format '%s.param-name: <values>", key, v, key, key)
			default:
				primitiveNodeInputs = append(primitiveNodeInputs, workflowbuilder.PrimitiveNodeInput{
					PrimitiveNodeID: key,
					Value:           value,
				})
			}
		}
	}

	return nodeInputs, primitiveNodeInputs, nil
}
