package execute

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gosuri/uilive"
	"github.com/spf13/cobra"
	"github.com/xlab/treeprint"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"math"
	"os"
	"os/signal"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
	"trickest-cli/cmd/create"
	"trickest-cli/cmd/download"
	"trickest-cli/cmd/list"
	"trickest-cli/types"
	"trickest-cli/util"
)

var (
	newWorkflowName   string
	configFile        string
	watch             bool
	showParams        bool
	executionMachines types.Bees
	hive              *types.Hive
	nodesToDownload   = make(map[string]download.NodeInfo, 0)
	allNodes          map[string]*types.TreeNode
	roots             []*types.TreeNode
	workflowYAML      string
	maxMachines       bool
)

// ExecuteCmd represents the execute command
var ExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute a workflow",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if util.SpaceName == "" {
			util.SpaceName = "Playground"
		}
		path := util.FormatPath()
		if path == "" || path == "Playground" {
			if len(args) == 0 {
				fmt.Println("Workflow name or path must be specified!")
				return
			}
			path = strings.Trim(args[0], "/")
		} else {
			if len(args) > 0 {
				fmt.Println("Please use either path or flag syntax for the platform objects.")
				return
			}
		}

		hive = util.GetHiveInfo()
		if hive == nil {
			return
		}
		var version *types.WorkflowVersionDetailed
		if workflowYAML != "" {
			version = readWorkflowYAMLandCreateVersion(workflowYAML, newWorkflowName, path)
		} else {
			version = prepareForExec(path)
		}
		if version == nil {
			fmt.Println("Couldn't find or create the workflow version!")
			os.Exit(0)
		}

		allNodes, roots = CreateTrees(version, false)
		executionMachines = version.MaxMachines
		if !maxMachines {
			setMachinesToMinimum(&executionMachines)
		}
		createRun(version.ID, watch, &executionMachines)
	},
}

func init() {
	ExecuteCmd.Flags().StringVar(&newWorkflowName, "name", "", "New workflow name (used when creating tool workflows or copying store workflows)")
	ExecuteCmd.Flags().StringVar(&configFile, "config", "", "YAML file for run configuration")
	ExecuteCmd.Flags().BoolVar(&watch, "watch", false, "Watch the execution running")
	ExecuteCmd.Flags().BoolVar(&showParams, "show-params", false, "Show parameters in the workflow tree")
	ExecuteCmd.Flags().StringVar(&workflowYAML, "file", "", "Workflow YAML file to execute")
	ExecuteCmd.Flags().BoolVar(&maxMachines, "max", false, "Use maximum number of machines for workflow execution")
}

func getToolScriptOrSplitterFromYAMLNode(node types.WorkflowYAMLNode) (*types.Tool, *types.Script, *types.Splitter) {
	var tool *types.Tool
	var script *types.Script
	var splitter *types.Splitter
	idSplit := strings.Split(node.ID, "-")
	if len(idSplit) == 1 {
		fmt.Println("Invalid node ID format: " + node.ID)
		os.Exit(0)
	}
	storeName := strings.TrimSuffix(node.ID, "-"+idSplit[len(idSplit)-1])

	if node.Script == nil {
		tools := list.GetTools(1, "", storeName)
		if tools == nil || len(tools) == 0 {
			splitter = getSplitter()
			if splitter == nil {
				fmt.Println("Couldn't find a tool named " + storeName + " in the store!")
				fmt.Println("Use \"trickest store list\" to see all available workflows and tools, " +
					"or search the store using \"trickest store search <name/description>\"")
				os.Exit(0)
			}
		} else {
			tool = &tools[0]
		}
	} else {
		script = getScriptByName(storeName)
		if script == nil {
			os.Exit(0)
		}
	}

	return tool, script, splitter
}

func setConnectedSplitters(version *types.WorkflowVersionDetailed, splitterIDs *map[string]string) {
	if splitterIDs == nil {
		tempMap := make(map[string]string, 0)
		splitterIDs = &tempMap
		for _, node := range version.Data.Nodes {
			if strings.HasPrefix(node.Name, "file-splitter") {
				(*splitterIDs)[node.Name] = node.Name
				continue
			}
			if node.WorkerConnected != nil {
				(*splitterIDs)[node.Name] = *node.WorkerConnected
			}
		}
		setConnectedSplitters(version, splitterIDs)
	} else {
		newConnectionFound := false
		for nodeName, splitterID := range *splitterIDs {
			for _, connection := range version.Data.Connections {
				if strings.Contains(connection.Source.ID, nodeName) {
					destinationNodeID := getNodeNameFromConnectionID(connection.Destination.ID)
					_, exists := (*splitterIDs)[destinationNodeID]
					if strings.HasPrefix(destinationNodeID, "file-splitter-") ||
						(strings.Contains(connection.Destination.ID, "folder") &&
							version.Data.Nodes[destinationNodeID].Script != nil) || exists {
						continue
					}
					version.Data.Nodes[destinationNodeID].WorkerConnected = &splitterID
					(*splitterIDs)[destinationNodeID] = splitterID
					newConnectionFound = true
				}
			}
		}
		if newConnectionFound {
			setConnectedSplitters(version, splitterIDs)
		}
	}
}

func nodeExists(nodes []types.WorkflowYAMLNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func readWorkflowYAMLandCreateVersion(fileName string, workflowName string, objectPath string) *types.WorkflowVersionDetailed {
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Couldn't open workflow file!")
		os.Exit(0)
	}
	defer file.Close()

	bytesData, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("Couldn't read workflow file!")
		os.Exit(0)
	}

	var wfNodes []types.WorkflowYAMLNode
	err = yaml.Unmarshal(bytesData, &wfNodes)
	if err != nil {
		fmt.Println("Couldn't unmarshal workflow YAML!")
		os.Exit(0)
	}

	space, project, workflow, _ := list.ResolveObjectPath(objectPath, true)
	if space == nil {
		fmt.Println("Space " + strings.Split(objectPath, "/")[0] + " doesn't exist!")
		os.Exit(0)
	}

	nodes := make(map[string]*types.Node, 0)
	connections := make([]types.Connection, 0)
	primitiveNodes := make(map[string]*types.PrimitiveNode, 0)

	stringInputsCnt := 0
	booleanInputsCnt := 0
	httpInputCnt := 0
	gitInputCnt := 0

	for _, node := range wfNodes {
		tool, script, splitter := getToolScriptOrSplitterFromYAMLNode(node)

		newNode := &types.Node{
			Name: node.ID,
			Meta: struct {
				Label       string `json:"label"`
				Coordinates struct {
					X float64 `json:"x"`
					Y float64 `json:"y"`
				} `json:"coordinates"`
			}{Label: node.Name, Coordinates: struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
			}{X: 0, Y: 0}},
		}

		machine := strings.ToLower(node.Machine)
		if machine == "" {
			machine = "large"
		}
		if node.Machine != "small" && node.Machine != "medium" && node.Machine != "large" {
			fmt.Println(node)
			fmt.Println("Invalid machine type!")
			os.Exit(0)
		}
		newNode.BeeType = machine

		if script != nil {
			newNode.ID = script.ID
			newNode.Script = &script.Script
			newNode.Type = script.Type
			outputs := struct {
				Folder *struct {
					Type  string `json:"type"`
					Order int    `json:"order"`
				} `json:"folder,omitempty"`
				File *struct {
					Type  string `json:"type"`
					Order int    `json:"order"`
				} `json:"file,omitempty"`
			}{
				Folder: &struct {
					Type  string `json:"type"`
					Order int    `json:"order"`
				}{
					Type:  script.Outputs.Folder.Type,
					Order: 0,
				},
				File: &struct {
					Type  string `json:"type"`
					Order int    `json:"order"`
				}{
					Type:  script.Outputs.File.Type,
					Order: 0,
				},
			}
			newNode.Outputs.File = outputs.File
			newNode.Outputs.Folder = outputs.Folder
			multi := true
			newNode.Inputs = map[string]*types.NodeInput{
				"file": {
					Type:  "FILE",
					Order: 0,
					Multi: &multi,
				},
				"folder": {
					Type:  "FOLDER",
					Order: 0,
					Multi: &multi,
				},
			}

			inputs, inputsExist := node.Inputs.(map[string]interface{})
			if inputsExist {
				filesVal, filesExist := inputs["file"]
				if !filesExist {
					filesVal, filesExist = inputs["files"]
				}
				if filesExist {
					files := filesVal.([]interface{})
					for _, value := range files {
						newPNode := types.PrimitiveNode{
							Coordinates: struct {
								X float64 `json:"x"`
								Y float64 `json:"y"`
							}{0, 0},
						}
						switch val := value.(type) {
						case string:
							if nodeExists(wfNodes, val) {
								connections = append(connections, types.Connection{
									Source: struct {
										ID string `json:"id"`
									}{
										ID: "output/" + val + "/file",
									},
									Destination: struct {
										ID string `json:"id"`
									}{
										ID: "input/" + node.ID + "/file/" + val,
									},
								})
								_, exists := newNode.Inputs[node.ID]
								if exists {
									fmt.Println(node)
									fmt.Println("Input with the same value already exists: " + val)
									os.Exit(0)
								}
								newNode.Inputs["file/"+val] = &types.NodeInput{
									Type:  "FILE",
									Order: 0,
									Value: "in/" + val + "/output.txt",
								}
								continue
							} else {
								if strings.HasPrefix(val, "http") {
									newPNode.Value = val
								} else {
									if _, err = os.Stat(val); errors.Is(err, os.ErrNotExist) {
										fmt.Println("A file named " + val + " doesn't exist!")
										os.Exit(0)
									}
									newPNode.Value = "trickest://file/" + val
								}
								newPNode.Type = "FILE"
								httpInputCnt++
								newPNode.Name = "http-input-" + strconv.Itoa(httpInputCnt)
								newPNode.TypeName = "URL"
								newPNode.Label = newPNode.Value.(string)
								primitiveNodes[newPNode.Name] = &newPNode
							}
						default:
							fmt.Println(node)
							fmt.Println("Unknown type for script file input! Use node ID or file.")
							os.Exit(0)
						}
						connection := types.Connection{
							Source: struct {
								ID string `json:"id"`
							}{
								ID: "output/" + newPNode.Name + "/output",
							},
							Destination: struct {
								ID string `json:"id"`
							}{
								ID: "input/" + node.ID + "/file/" + newPNode.Name,
							},
						}
						connections = append(connections, connection)
						newNode.Inputs["file/"+newPNode.Name] = &types.NodeInput{
							Type:  "FILE",
							Order: 0,
							Value: "in/" + newPNode.Name + "/" + path.Base(newPNode.Label),
						}
					}
				}
				foldersVal, foldersExist := inputs["folder"]
				if !foldersExist {
					foldersVal, foldersExist = inputs["folders"]
				}
				if foldersExist {
					files := foldersVal.([]interface{})
					for _, value := range files {
						newPNode := types.PrimitiveNode{
							Coordinates: struct {
								X float64 `json:"x"`
								Y float64 `json:"y"`
							}{0, 0},
						}
						switch val := value.(type) {
						case string:
							if nodeExists(wfNodes, val) {
								connections = append(connections, types.Connection{
									Source: struct {
										ID string `json:"id"`
									}{
										ID: "output/" + val + "/folder",
									},
									Destination: struct {
										ID string `json:"id"`
									}{
										ID: "input/" + node.ID + "/folder/" + val,
									},
								})
								_, exists := newNode.Inputs[node.ID]
								if exists {
									fmt.Println(node)
									fmt.Println("Input with the same value already exists: " + val)
									os.Exit(0)
								}
								newNode.Inputs["folder/"+val] = &types.NodeInput{
									Type:  "FOLDER",
									Order: 0,
									Value: "in/" + val + "/",
								}
								continue
							} else {
								if strings.HasPrefix(val, "http") && strings.HasSuffix(val, ".git") {
									newPNode.Value = val
								} else {
									fmt.Println("Folder input must be a complete repo URL with .git extension!")
									os.Exit(0)
								}
								newPNode.Type = "FOLDER"
								gitInputCnt++
								newPNode.Name = "git-input-" + strconv.Itoa(gitInputCnt)
								newPNode.TypeName = "GIT"
								newPNode.Label = newPNode.Value.(string)
								primitiveNodes[newPNode.Name] = &newPNode
							}
						default:
							fmt.Println(node)
							fmt.Println("Unknown type for script folder input! Use node ID or git repo URL.")
							os.Exit(0)
						}
						connection := types.Connection{
							Source: struct {
								ID string `json:"id"`
							}{
								ID: "output/" + newPNode.Name + "/output",
							},
							Destination: struct {
								ID string `json:"id"`
							}{
								ID: "input/" + node.ID + "/folder/" + newPNode.Name,
							},
						}
						connections = append(connections, connection)
						newNode.Inputs["folder/"+newPNode.Name] = &types.NodeInput{
							Type:  "FOLDER",
							Order: 0,
							Value: "in/" + newPNode.Name + "/",
						}
					}
				}
			}
		} else if tool != nil {
			newNode.ID = tool.ID
			newNode.Type = tool.Type
			newNode.Container = tool.Container
			newNode.Outputs.File = tool.Outputs.File
			newNode.Outputs.Folder = tool.Outputs.Folder
			newNode.OutputCommand = &tool.OutputCommand
			newNode.Inputs = make(map[string]*types.NodeInput, 0)

			inputs, ok := node.Inputs.(map[string]interface{})
			if !ok {
				fmt.Println("Invalid inputs format: ")
				fmt.Println(node)
				os.Exit(0)
			}
			for name, value := range inputs {
				toolInput, exists := tool.Inputs[name]
				if !exists {
					fmt.Println("Input parameter named " + name + " doesn't exist for " + tool.Name + "!")
					os.Exit(0)
				}

				newPNode := types.PrimitiveNode{
					Coordinates: struct {
						X float64 `json:"x"`
						Y float64 `json:"y"`
					}{0, 0},
				}

				switch val := value.(type) {
				case string:
					if toolInput.Type == "FILE" {
						if nodeExists(wfNodes, val) {
							connections = append(connections, types.Connection{
								Source: struct {
									ID string `json:"id"`
								}{
									ID: "output/" + val + "/file",
								},
								Destination: struct {
									ID string `json:"id"`
								}{
									ID: "input/" + node.ID + "/" + name,
								},
							})
							newNode.Inputs[name] = &types.NodeInput{
								Type:        toolInput.Type,
								Order:       0,
								Description: &toolInput.Description,
							}
							if toolInput.Command != "" {
								newNode.Inputs[name].Command = &toolInput.Command
							}
							if strings.HasPrefix(val, "file-splitter-") {
								workerConnected := true
								newNode.Inputs[name].Value = "in/" + val + ":item"
								newNode.Inputs[name].WorkerConnected = &workerConnected
							} else {
								newNode.Inputs[name].Value = "in/" + val + "/output.txt"
							}
							continue
						} else {
							newPNode.Type = "FILE"
							httpInputCnt++
							newPNode.Name = "http-input-" + strconv.Itoa(httpInputCnt)
							if strings.HasPrefix(val, "http") {
								newPNode.Value = val
							} else {
								newPNode.Value = "trickest://file/" + val
							}
						}
						newPNode.Label = newPNode.Value.(string)
						newPNode.TypeName = "URL"
					} else if toolInput.Type == "FOLDER" {
						if nodeExists(wfNodes, val) {
							connections = append(connections, types.Connection{
								Source: struct {
									ID string `json:"id"`
								}{
									ID: "output/" + val + "/folder",
								},
								Destination: struct {
									ID string `json:"id"`
								}{
									ID: "input/" + node.ID + "/" + name,
								},
							})
							newNode.Inputs[name] = &types.NodeInput{
								Type:        toolInput.Type,
								Order:       0,
								Command:     &toolInput.Command,
								Description: &toolInput.Description,
							}
							if strings.HasPrefix(val, "file-splitter-") {
								workerConnected := true
								newNode.Inputs[name].Value = "in/" + val + ":item"
								newNode.Inputs[name].WorkerConnected = &workerConnected
							} else {
								newNode.Inputs[name].Value = "in/" + val + "/"
							}
							continue
						} else {
							if strings.HasSuffix(val, ".git") {
								newPNode.Type = "FOLDER"
								gitInputCnt++
								newPNode.Name = "git-input-" + strconv.Itoa(gitInputCnt)
							} else {
								fmt.Println(name + " input parameter for " + tool.Name +
									" is a folder, use git repository (URL with .git extension) instead of " + val)
								os.Exit(0)
							}
						}
					} else {
						if nodeExists(wfNodes, val) {
							connections = append(connections, types.Connection{
								Source: struct {
									ID string `json:"id"`
								}{
									ID: "output/" + val + "/output",
								},
								Destination: struct {
									ID string `json:"id"`
								}{
									ID: "input/" + node.ID + "/" + name,
								},
							})
							workerConnected := true
							newNode.Inputs[name] = &types.NodeInput{
								Type:            toolInput.Type,
								Order:           0,
								Value:           "in/" + val + ":item",
								Command:         &toolInput.Command,
								Description:     &toolInput.Description,
								WorkerConnected: &workerConnected,
							}
							continue
						} else {
							newPNode.Label = val
							newPNode.Value = val
							stringInputsCnt++
							newPNode.Name = "string-input-" + strconv.Itoa(stringInputsCnt)
							newPNode.Type = "STRING"
							newPNode.TypeName = "STRING"
						}
					}
				case int:
					stringInputsCnt++
					newPNode.Name = "string-input-" + strconv.Itoa(stringInputsCnt)
					newPNode.Type = "STRING"
					newPNode.TypeName = "STRING"
					newPNode.Value = strconv.Itoa(val)
					newPNode.Label = strconv.Itoa(val)
				case bool:
					booleanInputsCnt++
					newPNode.Name = "boolean-input-" + strconv.Itoa(booleanInputsCnt)
					newPNode.Type = "BOOLEAN"
					newPNode.TypeName = "BOOLEAN"
					newPNode.Value = val
					newPNode.Label = strconv.FormatBool(val)
				default:
					fmt.Println(node)
					fmt.Println("Unknown type for tool input!")
					os.Exit(0)
				}

				in, exists := newNode.Inputs[name]
				if exists {
					fmt.Println("Input with the same name already exists!")
					fmt.Println("Name: " + name)
					fmt.Println("Values: ")
					fmt.Println(value)
					fmt.Println(in.Value)
					os.Exit(0)
				}
				newNode.Inputs[name] = &types.NodeInput{
					Type:        newPNode.Type,
					Order:       0,
					Command:     &toolInput.Command,
					Description: &toolInput.Description,
				}
				if newPNode.Type == "FILE" {
					newNode.Inputs[name].Value = "in/" + newPNode.Name + "/" + path.Base(newPNode.Value.(string))
				} else if newPNode.Type == "FOLDER" {
					newNode.Inputs[name].Value = "in/" + newPNode.Name + "/"
				} else {
					newNode.Inputs[name].Value = newPNode.Value
				}
				primitiveNodes[newPNode.Name] = &newPNode
				connections = append(connections, types.Connection{
					Source: struct {
						ID string `json:"id"`
					}{
						ID: "output/" + newPNode.Name + "/output",
					},
					Destination: struct {
						ID string `json:"id"`
					}{
						ID: "input/" + node.ID + "/" + name,
					},
				})
			}
			for name, input := range tool.Inputs {
				if _, exists := newNode.Inputs[name]; !exists {
					command := input.Command
					description := input.Description
					newNode.Inputs[name] = &types.NodeInput{
						Type:        input.Type,
						Order:       0,
						Command:     &command,
						Description: &description,
					}
				}
			}
		} else if splitter != nil {
			newNode.ID = splitter.ID
			newNode.Type = splitter.Type
			order := 0
			newNode.Outputs.Output = &struct {
				Type  string `json:"type"`
				Order *int   `json:"order,omitempty"`
			}{
				Type:  splitter.Outputs.Output.Type,
				Order: &order,
			}
			newNode.Inputs = make(map[string]*types.NodeInput, 0)
			inputs, ok := node.Inputs.([]interface{})
			if !ok {
				fmt.Println("Invalid inputs format: ")
				fmt.Println(node)
				os.Exit(0)
			}
			for _, value := range inputs {
				switch val := value.(type) {
				case string:
					if nodeExists(wfNodes, val) {
						connections = append(connections, types.Connection{
							Source: struct {
								ID string `json:"id"`
							}{
								ID: "output/" + val + "/file",
							},
							Destination: struct {
								ID string `json:"id"`
							}{
								ID: "input/" + node.ID + "/multiple/" + val,
							},
						})
						multi := true
						newNode.Inputs = map[string]*types.NodeInput{
							"multiple": {
								Type:  "FILE",
								Order: 0,
								Multi: &multi,
							},
						}
						newNode.Inputs["multiple/"+val] = &types.NodeInput{
							Type:  "FILE",
							Order: 0,
							Value: "in/" + val + "/output.txt",
						}
					} else {
						if _, err = os.Stat(val); errors.Is(err, os.ErrNotExist) {
							fmt.Println("A node with the given ID (" + val + ") doesn't exists in the workflow yaml!")
							fmt.Println("A file named " + val + " doesn't exist!")
							os.Exit(0)
						} else {
							httpInputCnt++
							newPNode := types.PrimitiveNode{
								Name:     "http-input-" + strconv.Itoa(httpInputCnt),
								Type:     "FILE",
								Label:    val,
								Value:    "trickest://file/" + val,
								TypeName: "URL",
								Coordinates: struct {
									X float64 `json:"x"`
									Y float64 `json:"y"`
								}{0, 0},
							}
							primitiveNodes[newPNode.Name] = &newPNode
							multi := true
							newNode.Inputs["multiple"] = &types.NodeInput{
								Type:  "FILE",
								Order: 0,
								Multi: &multi,
							}
							newNode.Inputs["multiple/"+newPNode.Name] = &types.NodeInput{
								Type:  "FILE",
								Order: 0,
								Value: "in/" + newPNode.Name + "/" + val,
							}
							connections = append(connections, types.Connection{
								Source: struct {
									ID string `json:"id"`
								}{
									ID: "output/" + newPNode.Name + "/output",
								},
								Destination: struct {
									ID string `json:"id"`
								}{
									ID: "input/" + node.ID + "/multiple/" + newPNode.Name,
								},
							})
						}
					}
				default:
					fmt.Println(node)
					fmt.Println("Unknown type for file splitter! Use node ID or file instead.")
					os.Exit(0)
				}
			}
		}
		nodes[node.ID] = newNode
	}

	workflowID := ""
	if workflow == nil {
		projectID := ""
		if project != nil {
			projectID = project.ID
		}
		if workflowName == "" {
			fmt.Println("Use --name flag when trying to create a new workflow.")
			os.Exit(0)
		}
		newWorkflow := create.CreateWorkflow(workflowName, "", space.ID, projectID, true)
		workflowID = newWorkflow.ID
	} else {
		workflowID = workflow.ID
	}
	version := &types.WorkflowVersionDetailed{
		WorkflowInfo: workflowID,
		Name:         nil,
		Data: struct {
			Nodes          map[string]*types.Node          `json:"nodes"`
			Connections    []types.Connection              `json:"connections"`
			PrimitiveNodes map[string]*types.PrimitiveNode `json:"primitiveNodes"`
		}{
			Nodes:          nodes,
			Connections:    connections,
			PrimitiveNodes: primitiveNodes,
		},
	}
	for _, pNode := range version.Data.PrimitiveNodes {
		if pNode.Type == "FILE" && strings.HasPrefix(pNode.Value.(string), "trickest://file/") {
			uploadFile(strings.TrimPrefix(pNode.Value.(string), "trickest://file/"))
		}
	}
	setConnectedSplitters(version, nil)
	generateNodesCoordinates(version)
	version = createNewVersion(version)
	return version
}

func WatchRun(runID string, nodesToDownload map[string]download.NodeInfo, timestampOnly bool, machines *types.Bees, showParameters bool) {
	const fmtStr = "%-12s %v\n"
	writer := uilive.New()
	writer.Start()
	defer writer.Stop()

	mutex := &sync.Mutex{}

	if !timestampOnly {
		go func() {
			defer mutex.Unlock()
			signalChannel := make(chan os.Signal, 1)
			signal.Notify(signalChannel, os.Interrupt)
			<-signalChannel

			mutex.Lock()
			_ = writer.Flush()
			writer.Stop()

			fmt.Println("The program will exit. Would you like to stop the remote execution? (Y/N)")
			var answer string
			for {
				_, _ = fmt.Scan(&answer)
				if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
					stopRun(runID)
					os.Exit(0)
				} else if strings.ToLower(answer) == "n" || strings.ToLower(answer) == "no" {
					os.Exit(0)
				}
			}
		}()
	}

	for {
		mutex.Lock()
		run := GetRunByID(runID)
		if run == nil {
			mutex.Unlock()
			break
		}

		out := ""
		out += fmt.Sprintf(fmtStr, "Name:", run.WorkflowName)
		out += fmt.Sprintf(fmtStr, "Status:", strings.ToLower(run.Status))
		availableBees := GetAvailableMachines()
		out += fmt.Sprintf(fmtStr, "Machines:", FormatMachines(machines, true)+
			" (currently available: "+FormatMachines(&availableBees, true)+")")
		out += fmt.Sprintf(fmtStr, "Created:", run.CreatedDate.In(time.Local).Format(time.RFC1123)+
			" ("+time.Since(run.CreatedDate).Round(time.Second).String()+" ago)")
		if run.Status != "PENDING" {
			if !run.StartedDate.IsZero() {
				out += fmt.Sprintf(fmtStr, "Started:", run.StartedDate.In(time.Local).Format(time.RFC1123)+
					" ("+time.Since(run.StartedDate).Round(time.Second).String()+" ago)")
			}
		}
		if run.Finished {
			if !run.CompletedDate.IsZero() {
				out += fmt.Sprintf(fmtStr, "Finished:", run.CompletedDate.In(time.Local).Format(time.RFC1123)+
					" ("+time.Since(run.CompletedDate).Round(time.Second).String()+" ago)")
			}
			out += fmt.Sprintf(fmtStr, "Duration:", (run.CompletedDate.Sub(run.StartedDate)).Round(time.Second).String())
		}
		if run.Status == "RUNNING" {
			out += fmt.Sprintf(fmtStr, "Duration:", time.Since(run.StartedDate).Round(time.Second).String())
		}

		subJobs := GetSubJobs(runID)
		for _, sj := range subJobs {
			allNodes[sj.NodeName].Status = strings.ToLower(sj.Status)
			allNodes[sj.NodeName].OutputStatus = strings.ReplaceAll(strings.ToLower(sj.OutputsStatus), "_", " ")
			if sj.Finished {
				allNodes[sj.NodeName].Duration = sj.FinishedDate.Sub(sj.StartedDate).Round(time.Second)
			} else {
				allNodes[sj.NodeName].Duration = time.Since(sj.StartedDate).Round(time.Second)
			}
		}

		trees := PrintTrees(roots, &allNodes, showParameters, true)
		out += "\n" + trees
		_, _ = fmt.Fprintln(writer, out)
		_ = writer.Flush()

		if timestampOnly {
			return
		}

		if run.Status == "COMPLETED" || run.Status == "STOPPED" || run.Status == "FAILED" {
			if len(nodesToDownload) > 0 {
				objectPath := run.SpaceName
				if run.ProjectName != "" {
					objectPath += "/" + run.ProjectName
				}
				objectPath += "/" + run.WorkflowName
				download.DownloadRunOutput(run, nodesToDownload, nil, objectPath)
			}
			mutex.Unlock()
			return
		}
		mutex.Unlock()
	}
}

func PrintTrees(roots []*types.TreeNode, allNodes *map[string]*types.TreeNode, showParameters bool, table bool) string {
	trees := ""
	for _, root := range roots {
		tree := printTree(root, nil, allNodes, showParameters)

		for _, node := range *allNodes {
			node.Printed = false
		}

		if !table {
			trees += tree
			continue
		}

		writerBuffer := new(bytes.Buffer)
		w := tabwriter.NewWriter(writerBuffer, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintf(w, "\tNODE\t STATUS\t DURATION\t OUTPUT\n")

		treeSplit := strings.Split(tree, "\n")
		for _, line := range treeSplit {
			if line != "" {
				if strings.Contains(line, "(") {
					lineSplit := strings.Split(line, "(")
					nodeName := strings.Trim(lineSplit[1], ")")
					node := (*allNodes)[nodeName]
					_, _ = fmt.Fprintf(w, "\t"+line+"\t"+node.Status+"\t"+
						node.Duration.Round(time.Second).String()+"\t"+node.OutputStatus+"\n")
				} else {
					_, _ = fmt.Fprintf(w, "\t"+line+"\t\t\t\n")
				}
			}
		}
		_ = w.Flush()
		trees += writerBuffer.String()
	}

	return trees
}

func printTree(node *types.TreeNode, branch *treeprint.Tree, allNodes *map[string]*types.TreeNode, showParameters bool) string {
	prefixSymbol := ""
	switch node.Status {
	case "pending":
		prefixSymbol = "\u23f3 " //⏳
	case "running":
		prefixSymbol = "\U0001f535 " //🔵
	case "succeeded":
		prefixSymbol = "\u2705 " //✅
	case "error", "failed":
		prefixSymbol = "\u274c " //❌
	}

	printValue := prefixSymbol + node.Label + " (" + node.NodeName + ")"
	if branch == nil {
		tree := treeprint.NewWithRoot(printValue)
		branch = &tree
	} else {
		childBranch := (*branch).AddBranch(printValue)
		branch = &childBranch
	}

	if showParameters {
		inputNames := make([]string, 0)
		for input := range *node.Inputs {
			inputNames = append(inputNames, input)
		}
		sort.Strings(inputNames)
		parameters := (*branch).AddBranch("parameters")
		for _, inputName := range inputNames {
			input := (*node.Inputs)[inputName]
			param := inputName + ": "
			if input.Value != nil {
				switch v := input.Value.(type) {
				case string:
					if strings.HasPrefix(v, "in/") {
						if strings.Contains(v, "/file-splitter-") {
							v = strings.TrimPrefix(v, "/in")
							v = strings.TrimSuffix(v, ":item")
						} else {
							fmt.Println(v)
							v = getNodeNameFromConnectionID(v)
						}
					}
					if strings.HasPrefix(param, "file/") || strings.HasPrefix(param, "folder/") {
						parameters.AddNode(v)
					} else {
						parameters.AddNode(param + v)
					}
				case int:
					parameters.AddNode(param + strconv.Itoa(v))
				case bool:
					parameters.AddNode(param + strconv.FormatBool(v))
				}
			}
		}
	}

	for _, child := range node.Children {
		if !(*allNodes)[node.NodeName].Printed {
			printTree(child, branch, allNodes, showParameters)
		}
	}

	(*allNodes)[node.NodeName].Printed = true

	return (*branch).String()
}

func CreateTrees(wfVersion *types.WorkflowVersionDetailed, includePrimitiveNodes bool) (map[string]*types.TreeNode, []*types.TreeNode) {
	allNodes = make(map[string]*types.TreeNode, 0)
	roots = make([]*types.TreeNode, 0)

	for _, node := range wfVersion.Data.Nodes {
		allNodes[node.Name] = &types.TreeNode{
			NodeName:     node.Name,
			Label:        node.Meta.Label,
			Inputs:       &node.Inputs,
			Status:       "pending",
			OutputStatus: "no outputs",
			Children:     make([]*types.TreeNode, 0),
			Parents:      make([]*types.TreeNode, 0),
		}
	}

	if includePrimitiveNodes {
		for _, node := range wfVersion.Data.PrimitiveNodes {
			allNodes[node.Name] = &types.TreeNode{
				NodeName: node.Name,
				Label:    node.Label,
			}
		}
	}

	for node := range wfVersion.Data.Nodes {
		for _, connection := range wfVersion.Data.Connections {
			if node == getNodeNameFromConnectionID(connection.Destination.ID) {
				child := getNodeNameFromConnectionID(connection.Source.ID)
				if childNode, exists := allNodes[child]; exists {
					if childNode.Parents == nil {
						childNode.Parents = make([]*types.TreeNode, 0)
					}
					childNode.Parents = append(childNode.Parents, allNodes[node])
					allNodes[node].Children = append(allNodes[node].Children, childNode)
				}
			}
		}
	}

	for node := range wfVersion.Data.Nodes {
		if allNodes[node].Parents == nil || len(allNodes[node].Parents) == 0 {
			roots = append(roots, allNodes[node])
		}
	}

	return allNodes, roots
}

func createToolWorkflow(wfName string, space *types.SpaceDetailed, project *types.Project, deleteProjectOnError bool,
	tool *types.Tool, primitiveNodes map[string]*types.PrimitiveNode, machine types.Bees) *types.WorkflowVersionDetailed {
	if tool == nil {
		fmt.Println("No tool specified, couldn't create a workflow!")
		os.Exit(0)
	}

	node := &types.Node{
		ID:   tool.ID,
		Name: tool.Name + "-1",
		Meta: struct {
			Label       string `json:"label"`
			Coordinates struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
			} `json:"coordinates"`
		}{Label: tool.Name, Coordinates: struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}{X: 0, Y: 0}},
		Type:          tool.Type,
		Container:     tool.Container,
		OutputCommand: &tool.OutputCommand,
	}
	node.Outputs.Folder = tool.Outputs.Folder
	node.Outputs.File = tool.Outputs.File
	switch {
	case machine.Small != nil:
		node.BeeType = "small"
	case machine.Medium != nil:
		node.BeeType = "medium"
	case machine.Large != nil:
		node.BeeType = "large"
	}

	inputs := make(map[string]*types.NodeInput, 0)
	for inputName, toolInput := range tool.Inputs {
		inputs[inputName] = &types.NodeInput{
			Type:        toolInput.Type,
			Order:       toolInput.Order,
			Command:     &toolInput.Command,
			Description: &toolInput.Description,
		}
		for _, primitiveNode := range primitiveNodes {
			if primitiveNode.ParamName == inputName {
				inputs[inputName].Value = primitiveNode.Value
				break
			}
		}
	}
	node.Inputs = inputs

	connections := make([]types.Connection, 0)
	distance := 400
	total := (len(primitiveNodes) - 1) * distance
	start := -total / 2

	pNodes := make([]string, 0)
	for pn := range primitiveNodes {
		pNodes = append(pNodes, pn)
	}
	sort.Strings(pNodes)

	for _, p := range pNodes {
		pNode := primitiveNodes[p]
		pNode.Coordinates.X = node.Meta.Coordinates.X - 2*float64(distance)
		pNode.Coordinates.Y = float64(start)
		start += distance

		connections = append(connections, types.Connection{
			Source: struct {
				ID string `json:"id"`
			}{ID: "output/" + pNode.Name + "/output"},
			Destination: struct {
				ID string `json:"id"`
			}{ID: "input/" + node.Name + "/" + pNode.ParamName},
		})
	}

	wfExists := false
	workflowID := ""
	projectID := ""
	if project != nil {
		projectID = project.ID
		for _, wf := range project.Workflows {
			if wf.Name == wfName {
				wfExists = true
				workflowID = wf.ID
				break
			}
		}
	} else {
		for _, wf := range space.Workflows {
			if wf.Name == wfName {
				wfExists = true
				workflowID = wf.ID
				break
			}
		}
	}

	if !wfExists {
		fmt.Println("Creating " + wfName + " workflow...")
		newWorkflow := create.CreateWorkflow(wfName, tool.Description, space.ID, projectID, deleteProjectOnError)
		workflowID = newWorkflow.ID
	}

	newVersion := &types.WorkflowVersionDetailed{
		WorkflowInfo: workflowID,
		Name:         nil,
		Data: struct {
			Nodes          map[string]*types.Node          `json:"nodes"`
			Connections    []types.Connection              `json:"connections"`
			PrimitiveNodes map[string]*types.PrimitiveNode `json:"primitiveNodes"`
		}{
			Nodes: map[string]*types.Node{
				node.Name: node,
			},
			Connections:    connections,
			PrimitiveNodes: primitiveNodes,
		},
	}

	for _, pNode := range newVersion.Data.PrimitiveNodes {
		if pNode.Type == "FILE" && strings.HasPrefix(pNode.Value.(string), "trickest://file/") {
			pNode.Label = uploadFile(strings.TrimPrefix(pNode.Value.(string), "trickest://file/"))
		}
	}
	newVersion = createNewVersion(newVersion)
	return newVersion
}

func prepareForExec(objectPath string) *types.WorkflowVersionDetailed {
	pathSplit := strings.Split(strings.Trim(objectPath, "/"), "/")
	var wfVersion *types.WorkflowVersionDetailed
	var primitiveNodes map[string]*types.PrimitiveNode
	projectCreated := false

	space, project, workflow, _ := list.ResolveObjectPath(objectPath, false)
	if space == nil {
		os.Exit(0)
	}
	if workflow == nil {
		wfName := pathSplit[len(pathSplit)-1]
		storeWorkflows := list.GetWorkflows("", "", wfName, true)
		if storeWorkflows != nil && len(storeWorkflows) > 0 {
			for _, wf := range storeWorkflows {
				if strings.ToLower(wf.Name) == strings.ToLower(wfName) {
					storeWfVersion := GetLatestWorkflowVersion(list.GetWorkflowByID(wf.ID))
					update := false
					var updatedWfVersion *types.WorkflowVersionDetailed
					if configFile == "" {
						executionMachines = storeWfVersion.MaxMachines
					} else {
						update, updatedWfVersion, primitiveNodes = readConfig(configFile, storeWfVersion, nil)
					}

					if project == nil {
						fmt.Println("Would you like to create a project named " + wfName +
							" and save the new workflow in there? (Y/N)")
						var answer string
						for {
							_, _ = fmt.Scan(&answer)
							if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
								project = create.CreateProjectIfNotExists(space, wfName)
								projectCreated = true
								break
							} else if strings.ToLower(answer) == "n" || strings.ToLower(answer) == "no" {
								break
							}
						}
					}

					if newWorkflowName == "" {
						newWorkflowName = wf.Name
					}
					copyDestination := space.Name
					if project != nil {
						copyDestination += "/" + project.Name
					}
					copyDestination += "/" + newWorkflowName
					fmt.Println("Copying " + wf.Name + " from the store to " + copyDestination)
					projID := ""
					if project != nil {
						projID = project.ID
					}
					newWorkflowID := copyWorkflow(space.ID, projID, wf.ID)
					if newWorkflowID == "" {
						fmt.Println("Couldn't copy workflow from the store!")
						os.Exit(0)
					}

					newWorkflow := list.GetWorkflowByID(newWorkflowID)
					if newWorkflow.Name != newWorkflowName {
						newWorkflow.Name = newWorkflowName
						updateWorkflow(newWorkflow, projectCreated)
					}

					wfVersion = GetLatestWorkflowVersion(newWorkflow)
					if update && updatedWfVersion != nil {
						for _, pNode := range primitiveNodes {
							if pNode.Type == "FILE" && strings.HasPrefix(pNode.Value.(string), "trickest://file/") {
								pNode.Label = uploadFile(strings.TrimPrefix(pNode.Value.(string), "trickest://file/"))
							}
						}
						updatedWfVersion.WorkflowInfo = newWorkflow.ID
						wfVersion = createNewVersion(updatedWfVersion)
					}
					return wfVersion
				}
			}
		}

		if configFile == "" {
			fmt.Println("You must provide config file to create a tool workflow!")
			os.Exit(0)
		}
		tools := list.GetTools(math.MaxInt, "", wfName)
		if tools == nil || len(tools) == 0 {
			fmt.Println("Couldn't find a workflow or tool named " + wfName + " in the store!")
			fmt.Println("Use \"trickest store list\" to see all available workflows and tools, " +
				"or search the store using \"trickest store search <name/description>\"")
			os.Exit(0)
		}
		_, _, primitiveNodes = readConfig(configFile, nil, &tools[0])

		if project == nil {
			fmt.Println("Would you like to create a project named " + wfName +
				" and save the new workflow in there? (Y/N)")
			var answer string
			for {
				_, _ = fmt.Scan(&answer)
				if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
					project = create.CreateProjectIfNotExists(space, tools[0].Name)
					projectCreated = true
					break
				} else if strings.ToLower(answer) == "n" || strings.ToLower(answer) == "no" {
					break
				}
			}
		}
		wfVersion = createToolWorkflow(newWorkflowName, space, project, projectCreated, &tools[0], primitiveNodes, executionMachines)

		return wfVersion
	} else {
		wfVersion = GetLatestWorkflowVersion(workflow)
		if configFile == "" {
			executionMachines = wfVersion.MaxMachines
		} else {
			update, updatedWfVersion, newPrimitiveNodes := readConfig(configFile, wfVersion, nil)
			if update {
				for _, pNode := range newPrimitiveNodes {
					if pNode.Type == "FILE" && strings.HasPrefix(pNode.Value.(string), "trickest://file/") {
						pNode.Label = uploadFile(strings.TrimPrefix(pNode.Value.(string), "trickest://file/"))
					}
				}
				wfVersion = createNewVersion(updatedWfVersion)
			}
		}
	}

	return wfVersion
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
				if n.Script != nil || strings.HasPrefix(id, "file-splitter") {
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
					if node.Script == nil && !strings.HasPrefix(node.Name, "file-splitter") &&
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
					node = getNodeByName(nameSplit[0], wfVersion)
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
					if strings.HasPrefix(node.ID, "file-splitter") {
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
				if node.Script == nil && !strings.HasPrefix(node.Name, "file-splitter") {
					oldParam, paramExists := node.Inputs[paramName]
					paramExists = paramExists && oldParam.Value != nil
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
					if strings.HasPrefix(val, "http") {
						newPNode.Value = val
					} else {
						if _, err := os.Stat(val); errors.Is(err, os.ErrNotExist) {
							fmt.Println("A file named " + val + " doesn't exist!")
							os.Exit(0)
						}
						newPNode.Value = "trickest://file/" + val
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
				if node == nil || (node.Script == nil && !strings.HasPrefix(node.Name, "file-splitter")) {
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
							if strings.HasPrefix(file, "http") {
								newPNode.Value = file
							} else {
								if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
									fmt.Println("A file named " + file + " doesn't exist!")
									os.Exit(0)
								}
								newPNode.Value = "trickest://file/" + file
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
				newPNode.ParamName = paramName
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
		isSplitter := strings.HasPrefix(node.Name, "file-splitter")
		if strings.HasSuffix(connection.Destination.ID, node.Name+"/"+paramName) ||
			(isSplitter && strings.HasSuffix(connection.Destination.ID, node.Name+"/multiple/"+source)) ||
			(node.Script != nil && (strings.HasSuffix(connection.Destination.ID,
				node.Name+"/"+strings.ToLower(newPNode.Type)+"/"+source))) {
			connectionFound = true
			primitiveNodeName := getNodeNameFromConnectionID(connection.Source.ID)
			var primitiveNode *types.PrimitiveNode
			primitiveNode, pNodeExists = wfVersion.Data.PrimitiveNodes[primitiveNodeName]
			if !(strings.HasPrefix(primitiveNodeName, "http-input") ||
				strings.HasPrefix(primitiveNodeName, "git-input") ||
				strings.HasPrefix(primitiveNodeName, "string-input") ||
				strings.HasPrefix(primitiveNodeName, "boolean-input")) {
				continue
			}
			if !pNodeExists {
				fmt.Println("Couldn't find primitive node: " + primitiveNodeName)
				os.Exit(0)
			}

			savedPNode, alreadyExists := (*newPrimitiveNodes)[primitiveNode.Name]
			if alreadyExists {
				if node.Script != nil || strings.HasPrefix(node.Name, "file-splitter") {
					continue
				} else if savedPNode.Value != newPNode.Value {
					processDifferentParamsForASinglePNode(*savedPNode, newPNode)
				}
			}
			newPNode.ParamName = paramName
			newPNode.Name = primitiveNode.Name
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
				} else if strings.HasPrefix(node.Name, "file-splitter") {
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
			break
		}
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

func readConfigOutputs(config *map[string]interface{}) map[string]download.NodeInfo {
	downloadNodes := make(map[string]download.NodeInfo)
	if outputs, exists := (*config)["outputs"]; exists && outputs != nil {
		outputsList, isList := outputs.([]interface{})
		if !isList {
			fmt.Println("Invalid outputs format! Use a list of strings.")
		}

		for _, node := range outputsList {
			nodeName, ok := node.(string)
			if ok {
				downloadNodes[nodeName] = download.NodeInfo{ToFetch: true, Found: false}
			} else {
				fmt.Print("Invalid output node name: ")
				fmt.Println(node)
			}
		}
	}
	return downloadNodes
}

func readConfigMachines(config *map[string]interface{}, isTool bool, maximumMachines *types.Bees) *types.Bees {
	if !isTool && maximumMachines == nil {
		fmt.Println("No maximum machines specified!")
		os.Exit(0)
	}

	execMachines := &types.Bees{}
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
				processMaxMachinesOverflow(maximumMachines)
			}
		}
	} else {
		if isTool {
			oneMachine := 1
			if maxMachines {
				return &types.Bees{Large: &oneMachine}
			} else {
				return &types.Bees{Small: &oneMachine}
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
