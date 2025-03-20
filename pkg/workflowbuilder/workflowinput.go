package workflowbuilder

import (
	"fmt"

	"github.com/trickest/trickest-cli/pkg/trickest"
)

type WorkflowInput interface {
	ApplyToWorkflowVersion(wfVersion *trickest.WorkflowVersion) error
}

// PrimitiveNodeInput represents a primitive node (string, boolean, file, or folder) input
type PrimitiveNodeInput struct {
	PrimitiveNodeID string
	Value           any
}

// NodeInput represents a node (tool, script, module, or splitter) input
type NodeInput struct {
	NodeID      string
	ParamValues map[string][]any
}

// ApplyToWorkflowVersion applies the primitive node input to the workflow version
func (input PrimitiveNodeInput) ApplyToWorkflowVersion(wfVersion *trickest.WorkflowVersion) error {
	if wfVersion == nil {
		return fmt.Errorf("workflow version is nil")
	}

	primitiveNode, exists := wfVersion.Data.PrimitiveNodes[input.PrimitiveNodeID]
	if !exists {
		return fmt.Errorf("primitive node %s not found", input.PrimitiveNodeID)
	}

	if err := setPrimitiveNodeValue(primitiveNode, input.Value); err != nil {
		return fmt.Errorf("failed to set primitive node value: %w", err)
	}

	if err := updatePrimitiveNodeReferences(wfVersion, primitiveNode); err != nil {
		return fmt.Errorf("failed to update primitive node references: %w", err)
	}

	return nil
}

// ApplyToWorkflowVersion applies the node input to the workflow version
func (input NodeInput) ApplyToWorkflowVersion(wfVersion *trickest.WorkflowVersion) error {
	if wfVersion == nil {
		return fmt.Errorf("workflow version is nil")
	}

	node, exists := wfVersion.Data.Nodes[input.NodeID]
	if !exists {
		return fmt.Errorf("node %s not found", input.NodeID)
	}

	for paramName, paramValues := range input.ParamValues {
		param, exists := node.Inputs[paramName]
		if !exists {
			return fmt.Errorf("parameter %s not found for node %s", paramName, input.NodeID)
		}

		// First clean up any existing primitive nodes for this parameter to make sure nodes don't keep old values
		// Any existing primitive nodes will be removed and replaced with new ones
		// This makes the process idempotent and clean as opposed to trying to update existing primitive nodes
		if err := cleanupNodeParam(wfVersion, input.NodeID, paramName); err != nil {
			return fmt.Errorf("failed to clean up existing primitive nodes: %w", err)
		}

		for _, paramValue := range paramValues {
			if err := setupNodeParam(wfVersion, input.NodeID, paramName, param.Type, paramValue); err != nil {
				return fmt.Errorf("failed to set up new primitive node: %w", err)
			}
		}
	}

	return nil
}
