package execute

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"trickest-cli/cmd/output"
	"trickest-cli/types"

	"gopkg.in/yaml.v3"
)

func readConfig(fileName string, wfVersion *types.WorkflowVersionDetailed, tool *types.Tool) (
	bool, *types.WorkflowVersionDetailed, map[string]*types.PrimitiveNode) {
	if wfVersion == nil && tool == nil {
		fmt.Println("No workflow or tool found for execution!")
		os.Exit(0)
	}
	if fileName == "" {
		return false, wfVersion, nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Couldn't open config file!")
		os.Exit(0)
	}
	defer file.Close()

	bytesData, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("Couldn't read config!")
		os.Exit(0)
	}

	var config map[string]interface{}
	err = yaml.Unmarshal(bytesData, &config)
	if err != nil {
		fmt.Println("Couldn't unmarshal config!")
		os.Exit(0)
	}

	if tool != nil {
		executionMachines = *readConfigMachines(&config, true, nil)
	} else {
		executionMachines = *readConfigMachines(&config, false, &wfVersion.MaxMachines)
	}
	nodesToDownload = readConfigOutputs(&config)
	updateNeeded, primitiveNodes := readConfigInputs(&config, wfVersion, tool)

	return updateNeeded, wfVersion, primitiveNodes
}

func readConfigInputs(config *map[string]interface{}, wfVersion *types.WorkflowVersionDetailed, tool *types.Tool) (
	bool, map[string]*types.PrimitiveNode) {
	updateNeeded := false
	newPrimitiveNodes := make(map[string]*types.PrimitiveNode, 0)
	if inputs, exists := (*config)["inputs"]; exists && inputs != nil {
		inputsList, isList := inputs.(map[string]interface{})
		if !isList {
			processInvalidInputStructure()
		}
		if tool != nil && len(inputsList) == 0 {
			fmt.Println("You must specify input parameters when creating a tool workflow!")
			os.Exit(0)
		}
		stringInputsCnt := 0
		booleanInputsCnt := 0
		httpInputCnt := 0
		gitInputCnt := 0

		for param, paramValue := range inputsList {
			var node *types.Node
			var paramName, nodeName string
			newPNode := types.PrimitiveNode{Name: "", Value: paramValue}
			if !strings.Contains(param, ".") {
				if tool != nil {
					paramName = param
					nodeName = tool.Name + "-1"
				} else if wfVersion != nil {
					if len(wfVersion.Data.Nodes) == 1 {
						for _, n := range wfVersion.Data.Nodes {
							node = n
							paramName = param
						}
					} else {
						node = getNodeByName(param, wfVersion)
					}
					isSplitter := strings.HasPrefix(node.Name, "file-splitter") || strings.HasPrefix(node.Name, "split-to-string")
					if node.Script == nil && !isSplitter &&
						len(wfVersion.Data.Nodes) > 1 {
						fmt.Println(param)
						fmt.Println("Node is not a script or a file splitter, use tool.param-name syntax instead!")
						os.Exit(0)
					}
					nodeName = node.Name
				} else {
					fmt.Println("No version or tool specified, can't read config inputs!")
					os.Exit(0)
				}
			} else {
				nameSplit := strings.Split(param, ".")
				if len(nameSplit) != 2 {
					fmt.Println("Invalid input parameter: " + param)
					fmt.Println("Use name or ID for scripts/splitter or (name or ID).param-name for tools.")
					os.Exit(0)
				}
				paramName = nameSplit[1]
				if wfVersion != nil {
					nodeName = nameSplit[0]
					nodeNameSplit := strings.Split(nodeName, "-")
					if _, err := strconv.Atoi(nodeNameSplit[len(nodeNameSplit)-1]); err != nil {
						nodeName += "-1"
					}
					node = getNodeByName(nodeName, wfVersion)
					if node.Script != nil {
						fmt.Println(param)
						fmt.Println("Node is a script, use the following syntax:")
						fmt.Println("<script name or ID>:")
						fmt.Println("   [file:")
						fmt.Println("      - <file name or URL>")
						fmt.Println("      ...\n   ]")
						fmt.Println("   [folder:")
						fmt.Println("      - <git repo URL>")
						fmt.Println("      ...\n   ]")
						os.Exit(0)
					}
					if strings.HasPrefix(node.Name, "file-splitter") || strings.HasPrefix(node.Name, "split-to-string") {
						fmt.Println(param)
						fmt.Println("Node is a file splitter, use the following syntax:")
						fmt.Println("<splitter name or ID>:")
						fmt.Println("   file:")
						fmt.Println("      - <file name or URL>")
						fmt.Println("      ...")
						os.Exit(0)
					}
					nodeName = node.Name
				} else {
					nodeName = tool.Name + "-1"
				}
			}

			inputType := ""
			if tool != nil {
				toolInput, paramExists := tool.Inputs[paramName]
				if !paramExists {
					fmt.Println("Input parameter " + paramName + " doesn't exist for tool named " + tool.Name + "!")
					os.Exit(0)
				}
				inputType = toolInput.Type
			} else {
				isSplitter := strings.HasPrefix(node.Name, "file-splitter") || strings.HasPrefix(node.Name, "split-to-string")
				if node.Script == nil && !isSplitter {
					oldParam, paramExists := node.Inputs[paramName]
					// paramExists = paramExists && oldParam.Value != nil
					if !paramExists {
						fmt.Println("Parameter " + paramName + " doesn't exist for node " + nodeName)
						os.Exit(0)
					}
					inputType = oldParam.Type
				}
			}

			switch val := newPNode.Value.(type) {
			case string:
				switch inputType {
				case "STRING":
					newPNode.Type = "STRING"
					newPNode.TypeName = "STRING"
					stringInputsCnt++
					newPNode.Name = "string-input-" + strconv.Itoa(stringInputsCnt)
					newPNode.Value = val
				case "FILE":
					if strings.HasPrefix(val, "http") || strings.HasPrefix(val, "trickest://file/") {
						newPNode.Value = val
					} else {
						if _, err := os.Stat(val); errors.Is(err, os.ErrNotExist) {
							fmt.Println("A file named " + val + " doesn't exist!")
							os.Exit(0)
						}
						newPNode.Value = "trickest://file/" + val
						trueVal := true
						newPNode.UpdateFile = &trueVal
					}
					newPNode.TypeName = "URL"
					httpInputCnt++
					newPNode.Name = "http-input-" + strconv.Itoa(httpInputCnt)
				case "FOLDER":
					if strings.HasPrefix(val, "http") && strings.HasSuffix(val, ".git") {
						newPNode.Value = val
					} else {
						fmt.Println("Folder input must be a complete repo URL with .git extension!")
						os.Exit(0)
					}
					newPNode.TypeName = "GIT"
					gitInputCnt++
					newPNode.Name = "git-input-" + strconv.Itoa(gitInputCnt)
				}
				newPNode.Type = inputType
			case int:
				newPNode.Type = inputType
				newPNode.TypeName = inputType
				stringInputsCnt++
				newPNode.Name = "string-input-" + strconv.Itoa(stringInputsCnt)
				newPNode.Value = strconv.Itoa(val)
			case bool:
				newPNode.Type = inputType
				newPNode.TypeName = inputType
				booleanInputsCnt++
				newPNode.Name = "boolean-input-" + strconv.Itoa(booleanInputsCnt)
				newPNode.Value = val
			case map[string]interface{}:
				isSplitter := strings.HasPrefix(node.Name, "file-splitter") || strings.HasPrefix(node.Name, "split-to-string")
				if node == nil || (node.Script == nil && !isSplitter) {
					fmt.Println(param + ": ")
					fmt.Println(val)
					fmt.Println("Invalid input type! Object inputs are used for scripts and splitters.")
					os.Exit(0)
				}
				inputFound := false
				filesVal, filesExist := val["file"]
				if !filesExist {
					filesVal, filesExist = val["files"]
				}
				if filesExist {
					inputFound = true
					files := filesVal.([]interface{})
					filePNodeNames := make([]string, 0)
					for _, input := range node.Inputs {
						if input.Type == "FILE" && input.Value != nil {
							valSplit := strings.Split(input.Value.(string), "/")
							if len(valSplit) >= 2 && strings.HasPrefix(valSplit[1], "http-input") {
								filePNodeNames = append(filePNodeNames, valSplit[1])
							}
						}
					}
					if len(filePNodeNames) != len(files) {
						fmt.Println(nodeName)
						fmt.Println("Number of file inputs doesn't match: ")
						fmt.Println("Existing: " + strconv.Itoa(len(filePNodeNames)))
						fmt.Println("Supplied: " + strconv.Itoa(len(files)))
						os.Exit(0)
					}
					for i, value := range files {
						switch file := value.(type) {
						case string:
							if strings.HasPrefix(file, "http") || strings.HasPrefix(file, "trickest://file/") {
								newPNode.Value = file
							} else {
								if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
									fmt.Println("A file named " + file + " doesn't exist!")
									os.Exit(0)
								}
								newPNode.Value = "trickest://file/" + file
								trueVal := true
								newPNode.UpdateFile = &trueVal
							}
							newPNode.Type = "FILE"
							httpInputCnt++
							newPNode.Name = filePNodeNames[i]
							newPNode.TypeName = "URL"
							newPNode.Label = newPNode.Value.(string)
							if wfVersion != nil {
								needsUpdate := addPrimitiveNodeFromConfig(wfVersion, &newPrimitiveNodes, newPNode, node, paramName)
								updateNeeded = updateNeeded || needsUpdate
							} else {
								if strings.ToLower(newPNode.Type) != strings.ToLower(inputType) {
									fmt.Println("Input parameter " + tool.Name + "." + paramName + " should be of type " +
										tool.Type + " instead of " + newPNode.Type + "!")
									os.Exit(0)
								}
								newPrimitiveNodes[newPNode.Name] = &newPNode
							}
						default:
							fmt.Println(file)
							fmt.Println("Unknown type for script file input! Use file name or URL.")
							os.Exit(0)
						}
					}
				}
				if node.Script != nil {
					foldersVal, foldersExist := val["folder"]
					if !foldersExist {
						foldersVal, foldersExist = val["folders"]
					}
					if foldersExist {
						inputFound = true
						folders := foldersVal.([]interface{})
						folderPNodeNames := make([]string, 0)
						for _, input := range node.Inputs {
							if input.Type == "FOLDER" {
								valSplit := strings.Split(input.Value.(string), "/")
								if len(valSplit) >= 2 && strings.HasPrefix(valSplit[1], "git-input") {
									folderPNodeNames = append(folderPNodeNames, valSplit[1])
								}
							}
						}
						if len(folderPNodeNames) != len(folders) {
							fmt.Println(nodeName)
							fmt.Println("Number of folder inputs doesn't match: ")
							fmt.Println("Existing: " + strconv.Itoa(len(folderPNodeNames)))
							fmt.Println("Supplied: " + strconv.Itoa(len(folders)))
							os.Exit(0)
						}
						for i, value := range folders {
							switch folder := value.(type) {
							case string:
								if strings.HasPrefix(folder, "http") && strings.HasSuffix(folder, ".git") {
									newPNode.Value = val
								} else {
									fmt.Println("Folder input must be a complete repo URL with .git extension!")
								}
								newPNode.Type = "FOLDER"
								gitInputCnt++
								newPNode.Name = folderPNodeNames[i]
								newPNode.TypeName = "GIT"
								newPNode.Label = newPNode.Value.(string)
								if wfVersion != nil {
									needsUpdate := addPrimitiveNodeFromConfig(wfVersion, &newPrimitiveNodes, newPNode, node, paramName)
									updateNeeded = updateNeeded || needsUpdate
								} else {
									if strings.ToLower(newPNode.Type) != strings.ToLower(inputType) {
										fmt.Println("Input parameter " + tool.Name + "." + paramName + " should be of type " +
											tool.Type + " instead of " + newPNode.Type + "!")
										os.Exit(0)
									}
									newPrimitiveNodes[newPNode.Name] = &newPNode
								}
							default:
								fmt.Println(folder)
								fmt.Println("Unknown type for script folder input! Use git repo URL.")
								os.Exit(0)
							}
						}
					}
				}
				if !inputFound {
					fmt.Println(val)
					fmt.Println("Invalid input object structure!")
				}
				continue
			default:
				newPNode.Type = "UNKNOWN"
			}
			if newPNode.Type == "BOOLEAN" {
				boolValue := newPNode.Value.(bool)
				newPNode.Label = strconv.FormatBool(boolValue)
			} else {
				newPNode.Label = newPNode.Value.(string)
			}

			if wfVersion != nil {
				needsUpdate := addPrimitiveNodeFromConfig(wfVersion, &newPrimitiveNodes, newPNode, node, paramName)
				updateNeeded = updateNeeded || needsUpdate
			} else {
				if strings.ToLower(newPNode.Type) != strings.ToLower(inputType) {
					fmt.Println("Input parameter " + tool.Name + "." + paramName + " should be of type " +
						tool.Type + " instead of " + newPNode.Type + "!")
					os.Exit(0)
				}
				newPNode.ParamName = &paramName
				newPrimitiveNodes[newPNode.Name] = &newPNode
			}
		}
	} else {
		if tool != nil {
			fmt.Println("You must specify input parameters when creating a tool workflow!")
			os.Exit(0)
		}
	}

	return updateNeeded, newPrimitiveNodes
}

func addPrimitiveNodeFromConfig(wfVersion *types.WorkflowVersionDetailed, newPrimitiveNodes *map[string]*types.PrimitiveNode,
	newPNode types.PrimitiveNode, node *types.Node, paramName string) bool {
	oldParam := node.Inputs[paramName]
	connectionFound := false
	pNodeExists := false
	updateNeeded := false
	for _, connection := range wfVersion.Data.Connections {
		source := getNodeNameFromConnectionID(connection.Source.ID)
		if !(strings.HasPrefix(source, "http-input") ||
			strings.HasPrefix(source, "git-input") ||
			strings.HasPrefix(source, "string-input") ||
			strings.HasPrefix(source, "boolean-input")) {
			continue
		}
		isSplitter := strings.HasPrefix(node.Name, "file-splitter") || strings.HasPrefix(node.Name, "split-to-string")
		if strings.HasSuffix(connection.Destination.ID, node.Name+"/"+paramName) ||
			(isSplitter && strings.HasSuffix(connection.Destination.ID, node.Name+"/multiple/"+source)) ||
			(node.Script != nil && (strings.HasSuffix(connection.Destination.ID,
				node.Name+"/"+strings.ToLower(newPNode.Type)+"/"+source))) {
			connectionFound = true
			primitiveNodeName := getNodeNameFromConnectionID(connection.Source.ID)
			if strings.HasSuffix(connection.Destination.ID, node.Name+"/"+paramName) && newPNode.Name != primitiveNodeName {
				// delete(wfVersion.Data.PrimitiveNodes, newPNode.Name)
				pNode, ok := wfVersion.Data.PrimitiveNodes[primitiveNodeName]
				if !ok {
					fmt.Println("Couldn't find primitive node: " + primitiveNodeName)
					os.Exit(0)
				}
				newPNode.Name = pNode.Name
				newPNode.Coordinates = pNode.Coordinates
				wfVersion.Data.PrimitiveNodes[primitiveNodeName] = &newPNode
			}
			var primitiveNode *types.PrimitiveNode
			primitiveNode, pNodeExists = wfVersion.Data.PrimitiveNodes[primitiveNodeName]
			if !pNodeExists {
				fmt.Println("Couldn't find primitive node: " + primitiveNodeName)
				os.Exit(0)
			}

			savedPNode, alreadyExists := (*newPrimitiveNodes)[primitiveNode.Name]
			if alreadyExists {
				if node.Script != nil || strings.HasPrefix(node.Name, "file-splitter") || strings.HasPrefix(node.Name, "split-to-string") {
					continue
				} else if savedPNode.Value != newPNode.Value {
					processDifferentParamsForASinglePNode(*savedPNode, newPNode)
				}
			}
			newPNode.Name = primitiveNode.Name
			newPNode.Coordinates = primitiveNode.Coordinates
			(*newPrimitiveNodes)[primitiveNode.Name] = &newPNode

			if (oldParam != nil && oldParam.Value != newPNode.Value) ||
				(newPNode.Type == "FILE" && strings.HasPrefix(newPNode.Value.(string), "trickest://file/")) {
				if newPNode.Type != primitiveNode.Type {
					processInvalidInputType(newPNode, *primitiveNode)
				}

				if node.Script != nil {
					for id, input := range node.Inputs {
						if id == "file/"+newPNode.Name {
							input.Value = "in/" + newPNode.Name + "/" + path.Base(newPNode.Value.(string))
							break
						}
					}
				} else if strings.HasPrefix(node.Name, "file-splitter") || strings.HasPrefix(node.Name, "split-to-string") {
					for id, input := range node.Inputs {
						if id == "multiple/"+newPNode.Name {
							input.Value = "in/" + newPNode.Name + "/" + path.Base(newPNode.Value.(string))
							break
						}
					}
				} else {
					node.Inputs[paramName].Value = newPNode.Value
				}

				if oldParam != nil {
					oldParam.Value = newPNode.Value
				}
				wfVersion.Name = nil
				updateNeeded = true
			}
			wfVersion.Data.PrimitiveNodes[primitiveNodeName] = &newPNode
			break
		}
	}

	if !pNodeExists {
		id := getAvailablePrimitiveNodeID(strings.ToLower(newPNode.Type), wfVersion.Data.Connections)
		nodeID := newPNode.Type + "-input-" + fmt.Sprint(id)
		nodeID = strings.ToLower(nodeID)

		pNode := createNewPrimitiveNode(newPNode.Type, newPNode.Value, id)

		wfVersion.Data.PrimitiveNodes[nodeID] = &pNode

		connection := createPrimitiveNodeConnection(nodeID, node.Name, paramName)
		wfVersion.Data.Connections = append(wfVersion.Data.Connections, connection)

		wfVersion.Data.Nodes[node.Name].Inputs[paramName].Value = newPNode.Value
		connectionFound = true
		pNodeExists = true
	}

	if !connectionFound {
		fmt.Println(node.Name + " is not connected to any input!")
		os.Exit(0)
	} else if !pNodeExists {
		fmt.Println(node.Meta.Label + " (" + node.Name + ") doesn't have a primitive node input!")
		os.Exit(0)
	}
	return updateNeeded
}

func readConfigOutputs(config *map[string]interface{}) map[string]output.NodeInfo {
	downloadNodes := make(map[string]output.NodeInfo)
	if outputs, exists := (*config)["outputs"]; exists && outputs != nil {
		outputsList, isList := outputs.([]interface{})
		if !isList {
			fmt.Println("Invalid outputs format! Use a list of strings.")
		}

		for _, node := range outputsList {
			nodeName, ok := node.(string)
			if ok {
				downloadNodes[nodeName] = output.NodeInfo{ToFetch: true, Found: false}
			} else {
				fmt.Print("Invalid output node name: ")
				fmt.Println(node)
			}
		}
	}
	return downloadNodes
}

func readConfigMachines(config *map[string]interface{}, isTool bool, maximumMachines *types.Machines) *types.Machines {
	if !isTool && maximumMachines == nil {
		fmt.Println("No maximum machines specified!")
		os.Exit(0)
	}

	execMachines := &types.Machines{}
	if machines, exists := (*config)["machines"]; exists && machines != nil {
		machinesList, ok := machines.(map[string]interface{})
		if !ok {
			processInvalidMachineStructure()
		}

		for name, val := range machinesList {
			isSmall := strings.ToLower(name) == "small"
			isMedium := strings.ToLower(name) == "medium"
			isLarge := strings.ToLower(name) == "large"

			if !isSmall && !isMedium && !isLarge {
				fmt.Print("Unrecognized machine: ")
				fmt.Print(name + ": ")
				fmt.Println(val)
				os.Exit(0)
			}

			var numberOfMachines *int
			switch value := val.(type) {
			case int:
				if value != 0 {
					if value < 0 {
						fmt.Println("Number of machines cannot be negative!")
						os.Exit(0)
					}
					if isTool && value > 1 {
						fmt.Println("You can specify only one machine of a single machine type for tool execution!")
						os.Exit(0)
					}
					numberOfMachines = &value
				}
			case string:
				if strings.ToLower(value) == "max" || strings.ToLower(value) == "maximum" || maxMachines {
					if isTool {
						oneMachine := 1
						if (isSmall && isMedium) || (isMedium && isLarge) || (isLarge && isSmall) {
							fmt.Println("You can specify only one machine of a single machine type for tool execution!")
							os.Exit(0)
						}
						numberOfMachines = &oneMachine
					} else {
						if isSmall {
							numberOfMachines = maximumMachines.Small
						} else if isMedium {
							numberOfMachines = maximumMachines.Medium
						} else if isLarge {
							numberOfMachines = maximumMachines.Large
						}
					}
				} else {
					processInvalidMachineString(value)
				}
			default:
				processInvalidMachineType(value)
			}

			if isSmall {
				execMachines.Small = numberOfMachines
			} else if isMedium {
				execMachines.Medium = numberOfMachines
			} else if isLarge {
				execMachines.Large = numberOfMachines
			}

		}

		if !isTool {
			maxMachinesOverflow := false
			if executionMachines.Small != nil {
				if maximumMachines.Small == nil || (*executionMachines.Small > *maximumMachines.Small) {
					maxMachinesOverflow = true
				}
			}
			if executionMachines.Medium != nil {
				if maximumMachines.Medium == nil || (*executionMachines.Medium > *maximumMachines.Medium) {
					maxMachinesOverflow = true
				}
			}
			if executionMachines.Large != nil {
				if maximumMachines.Large == nil || (*executionMachines.Large > *maximumMachines.Large) {
					maxMachinesOverflow = true
				}
			}

			if maxMachinesOverflow {
				processMaxMachinesOverflow(*maximumMachines)
			}
		}
	} else {
		if isTool {
			oneMachine := 1
			if maxMachines {
				return &types.Machines{Large: &oneMachine}
			} else {
				return &types.Machines{Small: &oneMachine}
			}
		} else {
			if maxMachines {
				return maximumMachines
			} else {
				execMachines = maximumMachines
				setMachinesToMinimum(execMachines)
			}
		}
	}

	return execMachines
}

func getNodeByName(name string, version *types.WorkflowVersionDetailed) *types.Node {
	var node *types.Node
	nodeName := name
	nameSplit := strings.Split(name, "-")
	_, err := strconv.Atoi(nameSplit[len(nameSplit)-1])
	if len(nameSplit) == 1 || (len(nameSplit) > 1 && err != nil) {
		nodeName += "-1"
	}
	var ok bool
	node, ok = version.Data.Nodes[nodeName]
	if !ok {
		labelCnt := 0
		toolNodeFound := false
		for id, n := range version.Data.Nodes {
			if n.Meta.Label == name {
				if n.Script != nil || strings.HasPrefix(id, "file-splitter") || strings.HasPrefix(id, "split-to-string") {
					node = n
					labelCnt++
					ok = true
				} else {
					toolNodeFound = true
				}
			}
		}
		if !ok {
			if toolNodeFound {
				fmt.Println("Incomplete input name for a tool node: " + name)
				fmt.Println("Use " + name + ".<parameter-name> instead.")
				os.Exit(0)
			}
			fmt.Println("Node doesn't exist: " + name)
			os.Exit(0)
		}
		if labelCnt > 1 {
			fmt.Println("Multiple nodes with the same label (" + name + "), use node IDs instead!")
			os.Exit(0)
		}
	}

	return node
}

func createNewPrimitiveNode(nodeType string, value interface{}, id int) types.PrimitiveNode {
	var node types.PrimitiveNode

	nodeType = strings.ToLower(nodeType)
	switch nodeType {
	case "file":
		node.Name = "http-input-" + fmt.Sprint(id)
	case "folder":
		node.Name = "git-input-" + fmt.Sprint(id)
	case "string":
		node.Name = "string-input-" + fmt.Sprint(id)
	case "boolean":
		node.Name = "boolean-input-" + fmt.Sprint(id)
	default:
		fmt.Println("Unknown primitive node type: " + nodeType)
		os.Exit(0)
	}

	node.Value = value
	node.Label = value.(string)
	node.Type = nodeType
	node.TypeName = nodeType

	return node
}

func getAvailablePrimitiveNodeID(nodeType string, connections []types.Connection) int {
	availableID := 1
	for _, connection := range connections {
		if strings.HasPrefix(connection.Source.ID, "output/"+nodeType+"-input-") {
			nodeID := strings.Split(connection.Source.ID, "/")[1]
			numericID, _ := strconv.Atoi(strings.Split(nodeID, "-")[2])
			if numericID >= availableID {
				availableID = numericID + 1
			}
		}
	}
	return availableID
}

func createPrimitiveNodeConnection(source, destinationNode, destinationInput string) types.Connection {
	var c types.Connection

	c.Source.ID = "output/" + source + "/output"
	c.Destination.ID = "input/" + destinationNode + "/" + destinationInput

	return c
}
