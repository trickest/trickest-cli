package workflowbuilder

import (
	"fmt"
	"strings"

	"github.com/trickest/trickest-cli/pkg/trickest"
)

// updatePrimitiveNodeReferences updates all references to the primitive node in connected nodes' inputs
//
// When primitive node is connected to a node, an input is created in the destination node with the primitive node's name and value
// For example, if string-input-1 is connected to string-to-file-1's "string" input, string-to-file-1 would look like this:
//
//	{
//	    "name": "string-to-file-1",
//	    "inputs": {
//	        "string": {
//	            "type": "STRING",
//	            "description": "Write strings to a file",
//	            "order": 0,
//	            "multi": true,
//	            "visible": true
//	        },
//	        "string/string-input-1": {
//	            "type": "STRING",
//	            "description": "Write strings to a file",
//	            "order": 0,
//	            "multi": true,
//	            "value": "example",
//	            "visible": true
//	        }
//	    }
//	}
func updatePrimitiveNodeReferences(wfVersion *trickest.WorkflowVersion, pNode *trickest.PrimitiveNode) error {
	if wfVersion == nil || pNode == nil {
		return fmt.Errorf("workflow version and primitive node cannot be nil")
	}

	// Find all connections where this primitive node is the source and update the destination node's input value
	for _, connection := range wfVersion.Data.Connections {
		sourceID := strings.TrimPrefix(connection.Source.ID, "output/")
		if !strings.HasPrefix(sourceID, pNode.Name) {
			continue
		}

		destTokens := strings.Split(strings.TrimPrefix(connection.Destination.ID, "input/"), "/")
		if len(destTokens) < 2 {
			return fmt.Errorf("connection destination is not formatted correctly: %s", connection.Destination.ID)
		}
		destNodeID := destTokens[0]
		destParam := destTokens[1]

		destNode, exists := wfVersion.Data.Nodes[destNodeID]
		if !exists {
			return fmt.Errorf("destination node %s does not exist", destNodeID)
		}

		if destNode.Inputs == nil {
			return fmt.Errorf("destination node %s inputs are nil", destNodeID)
		}

		if err := addNodeInputPrimitiveReference(destNode, pNode, destParam); err != nil {
			return err
		}
	}

	return nil
}

// addNodeInputPrimitiveReference adds a primitive node reference to a node's inputs list
func addNodeInputPrimitiveReference(node *trickest.Node, pNode *trickest.PrimitiveNode, inputName string) error {
	inputKey := fmt.Sprintf("%s/%s", inputName, pNode.Name)

	// Copy the original input definition and create a new input entry with the primitive node name included in the key
	originalInput, exists := node.Inputs[inputName]
	if !exists {
		return fmt.Errorf("input %s not found in node %s", inputName, node.Name)
	}

	// The original input, along with the new input, must be visible for the workflow to render
	originalInput.Visible = &[]bool{true}[0]

	node.Inputs[inputKey] = &trickest.NodeInput{
		Type:        originalInput.Type,
		Order:       originalInput.Order,
		Command:     originalInput.Command,
		Description: originalInput.Description,
		Multi:       originalInput.Multi,
		Visible:     &[]bool{true}[0],
	}

	switch pNode.Type {
	case "FILE":
		urlTokens := strings.Split(pNode.Value.(string), "/")
		fileName := urlTokens[len(urlTokens)-1]
		value := fmt.Sprintf("in/%s/%s", pNode.Name, fileName)
		node.Inputs[inputKey].Value = value
	case "FOLDER":
		value := fmt.Sprintf("in/%s/", pNode.Name)
		node.Inputs[inputKey].Value = value
	default:
		node.Inputs[inputKey].Value = pNode.Value
	}

	return nil
}

// removeNodeInputPrimitiveReference removes the primitive node reference from a node's inputs
func removeNodeInputPrimitiveReference(wfVersion *trickest.WorkflowVersion, nodeID string, pNodeID string, inputName string) error {
	if wfVersion == nil {
		return fmt.Errorf("workflow version is nil")
	}

	node, exists := wfVersion.Data.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	if node.Inputs == nil {
		return fmt.Errorf("node %s inputs are nil", nodeID)
	}

	inputKey := fmt.Sprintf("%s/%s", inputName, pNodeID)
	delete(node.Inputs, inputKey)

	return nil
}

// setupNodeParam sets up a primitive node, its connection, and its input reference
func setupNodeParam(wfVersion *trickest.WorkflowVersion, nodeID string, paramName string, paramType string, value any) error {
	if wfVersion == nil {
		return fmt.Errorf("workflow version is nil")
	}

	node, exists := wfVersion.Data.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	if _, exists := node.Inputs[paramName]; !exists {
		return fmt.Errorf("parameter %s not found for node %s", paramName, nodeID)
	}

	primitiveNode, err := addPrimitiveNode(wfVersion, paramType, value)
	if err != nil {
		return fmt.Errorf("failed to add primitive node: %w", err)
	}

	if err := addConnection(wfVersion, primitiveNode.Name, "output", nodeID, paramName); err != nil {
		// If connection fails, clean up the primitive node
		_ = removePrimitiveNode(wfVersion, primitiveNode.Name)
		return fmt.Errorf("failed to add connection: %w", err)
	}

	if err := addNodeInputPrimitiveReference(node, primitiveNode, paramName); err != nil {
		// If reference fails, clean up the connection and primitive node
		_ = removeConnection(wfVersion, primitiveNode.Name, "output", nodeID, paramName)
		_ = removePrimitiveNode(wfVersion, primitiveNode.Name)
		return fmt.Errorf("failed to add input reference: %w", err)
	}

	return nil
}

// cleanupNodeParam removes all primitive nodes, their connections, and their input references for a node parameter
func cleanupNodeParam(wfVersion *trickest.WorkflowVersion, nodeID string, paramName string) error {
	if wfVersion == nil {
		return fmt.Errorf("workflow version is nil")
	}

	primitiveNodeIDs, err := findPrimitiveNodesConnectedToParam(wfVersion, nodeID, paramName)
	if err != nil {
		return fmt.Errorf("failed to find primitive nodes: %w", err)
	}

	for _, pNodeID := range primitiveNodeIDs {
		if err := removePrimitiveNode(wfVersion, pNodeID); err != nil {
			return fmt.Errorf("failed to remove primitive node %s: %w", pNodeID, err)
		}

		if err := removeConnection(wfVersion, pNodeID, "output", nodeID, paramName); err != nil {
			return fmt.Errorf("failed to remove connection for primitive node %s: %w", pNodeID, err)
		}

		if err := removeNodeInputPrimitiveReference(wfVersion, nodeID, pNodeID, paramName); err != nil {
			return fmt.Errorf("failed to remove input reference for primitive node %s: %w", pNodeID, err)
		}
	}

	return nil
}
