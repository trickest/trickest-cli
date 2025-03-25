package workflowbuilder

import (
	"fmt"

	"github.com/trickest/trickest-cli/pkg/trickest"
)

// NodeLookupTable is a lookup table for nodes and primitive nodes in a workflow version that allows for quick lookup of node IDs by label or ID
type NodeLookupTable struct {
	Nodes          map[string]*trickest.Node
	PrimitiveNodes map[string]*trickest.PrimitiveNode
}

func BuildNodeLookupTable(wfVersion *trickest.WorkflowVersion) *NodeLookupTable {
	lookup := &NodeLookupTable{
		Nodes:          make(map[string]*trickest.Node),
		PrimitiveNodes: make(map[string]*trickest.PrimitiveNode),
	}

	for nodeID, node := range wfVersion.Data.Nodes {
		lookup.Nodes[nodeID] = node
		lookup.Nodes[node.Meta.Label] = node
	}

	for nodeID, node := range wfVersion.Data.PrimitiveNodes {
		lookup.PrimitiveNodes[nodeID] = node
		lookup.PrimitiveNodes[node.Label] = node
	}

	return lookup
}

func (lookup *NodeLookupTable) getNodeIDFromReference(ref string) (string, error) {
	if node, exists := lookup.Nodes[ref]; exists {
		return node.Name, nil
	}
	return "", fmt.Errorf("node reference '%s' was not found in node IDs or labels", ref)
}

func (lookup *NodeLookupTable) getPrimitiveNodeIDFromReference(ref string) (string, error) {
	if node, exists := lookup.PrimitiveNodes[ref]; exists {
		return node.Name, nil
	}
	return "", fmt.Errorf("primitive node reference '%s' was not found in primitive node IDs or labels", ref)
}

func (lookup *NodeLookupTable) ResolveInputs(inputs *Inputs) error {
	for i := range inputs.NodeInputs {
		nodeID, err := lookup.getNodeIDFromReference(inputs.NodeInputs[i].NodeID)
		if err != nil {
			return fmt.Errorf("failed to resolve node reference '%s': %w", inputs.NodeInputs[i].NodeID, err)
		}
		inputs.NodeInputs[i].NodeID = nodeID
	}

	for i := range inputs.PrimitiveNodeInputs {
		nodeID, err := lookup.getPrimitiveNodeIDFromReference(inputs.PrimitiveNodeInputs[i].PrimitiveNodeID)
		if err != nil {
			return fmt.Errorf("failed to resolve primitive node reference '%s': %w", inputs.PrimitiveNodeInputs[i].PrimitiveNodeID, err)
		}
		inputs.PrimitiveNodeInputs[i].PrimitiveNodeID = nodeID
	}

	return nil
}

func (lookup *NodeLookupTable) GetNodeInputType(nodeID string, paramName string) (string, error) {
	node, exists := lookup.Nodes[nodeID]
	if !exists {
		return "", fmt.Errorf("node '%s' not found", nodeID)
	}

	param, exists := node.Inputs[paramName]
	if !exists {
		return "", fmt.Errorf("parameter '%s' not found for node '%s'", paramName, nodeID)
	}
	return param.Type, nil
}

func (lookup *NodeLookupTable) GetPrimitiveNodeInputType(nodeID string) (string, error) {
	node, exists := lookup.PrimitiveNodes[nodeID]
	if !exists {
		return "", fmt.Errorf("primitive node '%s' not found", nodeID)
	}
	return node.Type, nil
}
