package execute

import (
	"sort"
	"trickest-cli/types"
)

func treeHeight(root *types.TreeNode) int {
	if root.Children == nil || len(root.Children) == 0 {
		return 0
	}

	maxHeight := 0
	for _, child := range root.Children {
		newHeight := treeHeight(child)
		if newHeight > maxHeight {
			maxHeight = newHeight
		}
	}
	return maxHeight + 1
}

func adjustChildrenHeight(root *types.TreeNode, nodesPerHeight *map[int][]*types.TreeNode) {
	if root.Parents == nil || len(root.Parents) == 0 {
		(*nodesPerHeight)[root.Height] = append((*nodesPerHeight)[root.Height], root)
	}
	if root.Children == nil || len(root.Children) == 0 {
		return
	}
	for _, child := range root.Children {
		child.Height = root.Height - 1
		found := false
		for _, node := range (*nodesPerHeight)[child.Height] {
			if node.Name == child.Name {
				found = true
				break
			}
		}
		if !found {
			(*nodesPerHeight)[child.Height] = append((*nodesPerHeight)[child.Height], child)
		}
		adjustChildrenHeight(child, nodesPerHeight)
	}
}

func generateNodesCoordinates(version *types.WorkflowVersionDetailed) {
	treesNodes, rootNodes := CreateTrees(version, true)
	for _, node := range treesNodes {
		node.Height = treeHeight(node)
	}

	nodesPerHeight := make(map[int][]*types.TreeNode, 0)
	maxRootHeight := 0
	for _, node := range rootNodes {
		if node.Height > maxRootHeight {
			maxRootHeight = node.Height
		}
	}
	for _, node := range rootNodes {
		adjustChildrenHeight(node, &nodesPerHeight)
	}
	for _, node := range rootNodes {
		if node.Height == maxRootHeight {
			adjustChildrenHeight(node, &nodesPerHeight)
		}
	}
	for _, root := range rootNodes {
		maxChildHeight := 0
		for _, child := range root.Children {
			if child.Height > maxChildHeight {
				maxChildHeight = child.Height
			}
		}
		if root.Height <= maxChildHeight {
			root.Height = maxChildHeight + 1
			if root.Height > maxRootHeight {
				maxRootHeight = root.Height
			}
			adjustChildrenHeight(root, &nodesPerHeight)
		}
	}
	for _, node := range treesNodes {
		if node.Parents != nil && len(node.Parents) > 0 {
			minHeightParent := node.Parents[0]
			for _, parent := range node.Parents {
				if parent.Height < minHeightParent.Height {
					minHeightParent = parent
				}
			}
			if minHeightParent.Height <= node.Height {
				adjustChildrenHeight(minHeightParent, &nodesPerHeight)
			}
		}
	}
	nodesPerHeight = make(map[int][]*types.TreeNode, 0)
	for _, node := range treesNodes {
		if _, exists := nodesPerHeight[node.Height]; !exists {
			nodesPerHeight[node.Height] = make([]*types.TreeNode, 0)
		}
		nodesPerHeight[node.Height] = append(nodesPerHeight[node.Height], node)
	}

	maxInputsPerHeight := make(map[int]int, 0)
	for height, nodes := range nodesPerHeight {
		maxInputs := 0
		for _, node := range nodes {
			if version.Data.Nodes[node.Name] != nil && len(version.Data.Nodes[node.Name].Inputs) > maxInputs {
				maxInputs = len(version.Data.Nodes[node.Name].Inputs)
			}
		}
		maxInputsPerHeight[height] = maxInputs
	}

	distance := 400
	X := float64(0)
	for height := 0; height < len(nodesPerHeight); height++ {
		nodes := nodesPerHeight[height]
		sort.SliceStable(nodes, func(i, j int) bool {
			return nodes[i].Name < nodes[j].Name
		})
		total := (len(nodes) - 1) * distance
		start := -total / 2
		nodeSizeIndent := float64(distance * (maxInputsPerHeight[height] / 15))
		previousHeightNodeSizeIndent := float64(0)
		if height-1 >= 0 {
			previousHeightNodeSizeIndent = float64(distance * (maxInputsPerHeight[height-1] / 10))
		}
		for i, node := range nodes {
			if version.Data.Nodes[node.Name] != nil {
				version.Data.Nodes[node.Name].Meta.Coordinates.X = X
				if i == 0 && height > 0 {
					version.Data.Nodes[node.Name].Meta.Coordinates.X += nodeSizeIndent
				}
				version.Data.Nodes[node.Name].Meta.Coordinates.X += previousHeightNodeSizeIndent
				version.Data.Nodes[node.Name].Meta.Coordinates.Y = 1.2 * float64(start)
				start += distance
				if i+1 < len(nodes) && version.Data.Nodes[nodes[i+1].Name] != nil &&
					len(version.Data.Nodes[nodes[i+1].Name].Inputs) == maxInputsPerHeight[height] {
					start += int(nodeSizeIndent)
				}
				if len(version.Data.Nodes[node.Name].Inputs) == maxInputsPerHeight[height] {
					start += int(nodeSizeIndent)
				}
			} else if version.Data.PrimitiveNodes[node.Name] != nil {
				version.Data.PrimitiveNodes[node.Name].Coordinates.X = X
				if i == 0 && height > 0 {
					version.Data.PrimitiveNodes[node.Name].Coordinates.X += nodeSizeIndent
				}
				version.Data.PrimitiveNodes[node.Name].Coordinates.X += previousHeightNodeSizeIndent
				version.Data.PrimitiveNodes[node.Name].Coordinates.Y = 1.2 * float64(start)
				start += distance
				if i+1 < len(nodes) && version.Data.Nodes[nodes[i+1].Name] != nil &&
					len(version.Data.Nodes[nodes[i+1].Name].Inputs) == maxInputsPerHeight[height] {
					start += int(nodeSizeIndent)
				}
			}
			if i == len(nodes)-1 {
				X += float64(distance * 2)
				X += previousHeightNodeSizeIndent
			}
		}
	}
}
