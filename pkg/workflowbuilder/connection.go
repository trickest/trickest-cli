package workflowbuilder

import (
	"fmt"
	"strings"

	"github.com/trickest/trickest-cli/pkg/trickest"
)

// addConnection adds a connection between two nodes to the workflow version
func addConnection(wfVersion *trickest.WorkflowVersion, sourceName, sourcePort, destinationName, destinationPort string) error {
	if wfVersion == nil {
		return fmt.Errorf("workflow version is nil")
	}

	wfVersion.Data.Connections = append(wfVersion.Data.Connections, trickest.Connection{
		Source: trickest.ConnectionEndpoint{
			ID: fmt.Sprintf("output/%s/%s", sourceName, sourcePort),
		},
		Destination: trickest.ConnectionEndpoint{
			ID: fmt.Sprintf("input/%s/%s/%s", destinationName, destinationPort, sourceName),
		},
	})

	return nil
}

// removeConnection removes a connection between nodes from the workflow version
func removeConnection(wfVersion *trickest.WorkflowVersion, sourceName, sourcePort, destinationName, destinationPort string) error {
	if wfVersion == nil {
		return fmt.Errorf("workflow version is nil")
	}

	sourceID := fmt.Sprintf("output/%s/%s", sourceName, sourcePort)
	destinationID := fmt.Sprintf("input/%s/%s/%s", destinationName, destinationPort, sourceName)

	for i, connection := range wfVersion.Data.Connections {
		if connection.Source.ID == sourceID && connection.Destination.ID == destinationID {
			// Remove the connection by appending all connections except the one at index i
			wfVersion.Data.Connections = append(wfVersion.Data.Connections[:i], wfVersion.Data.Connections[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("connection not found")
}

// findPrimitiveNodesConnectedToParam finds all primitive nodes connected to a specific node's parameter
func findPrimitiveNodesConnectedToParam(wfVersion *trickest.WorkflowVersion, nodeID string, paramName string) ([]string, error) {
	if wfVersion == nil {
		return nil, fmt.Errorf("workflow version is nil")
	}

	var primitiveNodeIDs []string
	for _, connection := range wfVersion.Data.Connections {
		destTokens := strings.Split(strings.TrimPrefix(connection.Destination.ID, "input/"), "/")
		if len(destTokens) < 2 {
			continue
		}
		destNodeID := destTokens[0]
		destParam := destTokens[1]

		if destNodeID == nodeID && destParam == paramName {
			sourceID := strings.TrimSuffix(strings.TrimPrefix(connection.Source.ID, "output/"), "/output")
			// Check if source is a primitive node by looking at the prefix
			if strings.HasPrefix(sourceID, "string-input-") ||
				strings.HasPrefix(sourceID, "boolean-input-") ||
				strings.HasPrefix(sourceID, "http-input-") ||
				strings.HasPrefix(sourceID, "git-input-") {
				primitiveNodeIDs = append(primitiveNodeIDs, sourceID)
			}
		}
	}

	return primitiveNodeIDs, nil
}
