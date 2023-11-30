package execute

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/trickest/trickest-cli/cmd/create"
	"github.com/trickest/trickest-cli/cmd/list"
	"github.com/trickest/trickest-cli/cmd/output"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"

	"github.com/google/uuid"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	newWorkflowName      string
	configFile           string
	watch                bool
	showParams           bool
	executionMachines    types.Machines
	fleet                *types.Fleet
	nodesToDownload      = make(map[string]output.NodeInfo, 0)
	allNodes             map[string]*types.TreeNode
	roots                []*types.TreeNode
	workflowYAML         string
	maxMachines          bool
	machineConfiguration string
	downloadAllNodes     bool
	outputsDirectory     string
	outputNodesFlag      string
	ci                   bool
	createProject        bool
	fleetName            string
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
		if path == "" {
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

		fleet = util.GetFleetInfo(fleetName)
		if fleet == nil {
			return
		}
		var version *types.WorkflowVersionDetailed
		if workflowYAML != "" {
			// Executing from a file
			version = readWorkflowYAMLandCreateVersion(workflowYAML, newWorkflowName, path)
		} else {
			// Executing an existing workflow or copying from library
			version = prepareForExec(path)
		}
		if version == nil {
			fmt.Println("Couldn't find or create the workflow version!")
			os.Exit(0)
		}

		allNodes, roots = CreateTrees(version, false)

		if maxMachines {
			executionMachines = version.MaxMachines
		} else if machineConfiguration != "" {
			machines, err := parseMachineConfiguration(machineConfiguration)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			if len(fleet.Machines) == 3 {
				// 3 types of machines: small, medium, and large
				executionMachines = machines

				if machines.Default != nil {
					fmt.Printf("Error: you need to use the small-medium-large format to specify the numbers of machines (e.g. 1-2-3)")
					os.Exit(1)
				}
			} else {
				// 1 type of machine
				executionMachines, err = handleSingleMachineType(*fleet, machines)
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
			}
		} else {
			executionMachines = setMachinesToMinimum(version.MaxMachines)
		}

		outputNodes := make([]string, 0)
		if outputNodesFlag != "" {
			outputNodes = strings.Split(outputNodesFlag, ",")
		}

		if !maxMachinesTypeCompatible(executionMachines, version.MaxMachines) {
			fmt.Println("Workflow maximum machines types are not compatible with config machines!")
			fmt.Println("Workflow max machines: " + FormatMachines(version.MaxMachines, true))
			fmt.Println("Config machines: " + FormatMachines(executionMachines, true))
			os.Exit(0)
		}

		createRun(version.ID, fleet.ID, watch, outputNodes, outputsDirectory)
	},
}

func parseMachineConfiguration(config string) (types.Machines, error) {
	pattern := `^\d+-\d+-\d+$`
	regex := regexp.MustCompile(pattern)

	if regex.MatchString(config) {
		// 3 types of machines, 3 hyphen-delimited inputs
		parts := strings.Split(config, "-")

		if len(parts) != 3 {
			return types.Machines{}, fmt.Errorf("invalid number of machines in machine configuration \"%s\"", config)
		}

		sizes := make([]int, 3)
		for index, part := range parts {
			if size, err := strconv.Atoi(part); err == nil {
				sizes[index] = size
			} else {
				return types.Machines{}, fmt.Errorf("invalid machine configuration \"%s\"", config)
			}
		}

		return types.Machines{
			// sizes = [small, medium, large]
			Small:  &sizes[0],
			Medium: &sizes[1],
			Large:  &sizes[2],
		}, nil
	}

	// One type of machine
	val, err := strconv.Atoi(config)
	if err != nil {
		return types.Machines{}, fmt.Errorf("invalid machine configuration \"%s\"", config)
	}

	return types.Machines{Default: &val}, nil
}

func handleSingleMachineType(fleet types.Fleet, machines types.Machines) (types.Machines, error) {
	var configMachines types.Machines

	var defaultOrSelfHosted int
	if machines.Default != nil {
		defaultOrSelfHosted = *machines.Default
	} else {
		// Backward-compatibility with the small-medium-large format
		defaultOrSelfHosted = *machines.Small + *machines.Medium + *machines.Large
		fmt.Printf("Warning: You have one type of machine in your fleet. %d identical or self-hosted machines will be used.\n", defaultOrSelfHosted)
	}

	if defaultOrSelfHosted == 0 {
		return types.Machines{}, fmt.Errorf("cannot run the workflow on %d machines", defaultOrSelfHosted)
	}

	if fleet.Type == "MANAGED" {
		configMachines.Default = &defaultOrSelfHosted
	} else if fleet.Type == "HOSTED" {
		configMachines.SelfHosted = &defaultOrSelfHosted
	} else {
		return types.Machines{}, fmt.Errorf("unsupported format. Use small-medium-large (e.g., 0-0-3)")
	}
	return configMachines, nil
}

func init() {
	ExecuteCmd.Flags().StringVar(&newWorkflowName, "set-name", "", "Set workflow name")
	ExecuteCmd.Flags().StringVar(&configFile, "config", "", "YAML file for run configuration")
	ExecuteCmd.Flags().BoolVar(&watch, "watch", false, "Watch the execution running")
	ExecuteCmd.Flags().BoolVar(&showParams, "show-params", false, "Show parameters in the workflow tree")
	// ExecuteCmd.Flags().StringVar(&workflowYAML, "file", "", "Workflow YAML file to execute")
	ExecuteCmd.Flags().BoolVar(&maxMachines, "max", false, "Use maximum number of machines for workflow execution")
	ExecuteCmd.Flags().StringVar(&machineConfiguration, "machines", "", "Specify the number of machines. Use one value for default/self-hosted machines (--machines 3) or three values for small-medium-large (--machines 1-1-1)")
	ExecuteCmd.Flags().BoolVar(&downloadAllNodes, "output-all", false, "Download all outputs when the execution is finished")
	ExecuteCmd.Flags().StringVar(&outputNodesFlag, "output", "", "A comma separated list of nodes which outputs should be downloaded when the execution is finished")
	ExecuteCmd.Flags().StringVar(&outputsDirectory, "output-dir", "", "Path to directory which should be used to store outputs")
	ExecuteCmd.Flags().BoolVar(&ci, "ci", false, "Run in CI mode (in-progreess executions will be stopped when the CLI is forcefully stopped - if not set, you will be asked for confirmation)")
	ExecuteCmd.Flags().BoolVar(&createProject, "create-project", false, "If the project doesn't exist, create it using the project flag as its name (or workflow name if not set)")
	ExecuteCmd.Flags().StringVar(&fleetName, "fleet", "", "The name of the fleet to use to execute the workflow")
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

	var wf types.WorkflowYAML
	err = yaml.Unmarshal(bytesData, &wf)
	if err != nil {
		fmt.Println("Couldn't unmarshal workflow YAML!")
		os.Exit(0)
	}

	if workflowName == "" {
		workflowName = wf.Name
	}

	space, project, workflow, _ := util.ResolveObjectPath(objectPath, true, false)
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

	for _, node := range wf.Steps {
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
			if node.Script != nil {
				newNode.Script.Source = *node.Script
			}
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
							if nodeExists(wf.Steps, val) {
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
								if strings.HasPrefix(val, "http") || strings.HasPrefix(val, "trickest://file/") {
									newPNode.Value = val
								} else {
									if _, err = os.Stat(val); errors.Is(err, os.ErrNotExist) {
										fmt.Println("A file named " + val + " doesn't exist!")
										os.Exit(0)
									}
									newPNode.Value = "trickest://file/" + val
									trueVal := true
									newPNode.UpdateFile = &trueVal
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
							if nodeExists(wf.Steps, val) {
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
						if nodeExists(wf.Steps, val) {
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
							if strings.HasPrefix(val, "file-splitter-") || strings.HasPrefix(val, "split-to-string-") {
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
						if nodeExists(wf.Steps, val) {
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
							if strings.HasPrefix(val, "file-splitter-") || strings.HasPrefix(val, "split-to-string-") {
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
						if nodeExists(wf.Steps, val) {
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
					if nodeExists(wf.Steps, val) {
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
						if _, err = os.Stat(val); errors.Is(err, os.ErrNotExist) && !strings.HasPrefix(val, "trickest://file/") {
							fmt.Println("A node with the given ID (" + val + ") doesn't exists in the workflow yaml!")
							fmt.Println("A file named " + val + " doesn't exist!")
							os.Exit(0)
						} else {
							httpInputCnt++
							newPNode := types.PrimitiveNode{
								Name:     "http-input-" + strconv.Itoa(httpInputCnt),
								Type:     "FILE",
								Label:    val,
								TypeName: "URL",
								Coordinates: struct {
									X float64 `json:"x"`
									Y float64 `json:"y"`
								}{0, 0},
							}
							if strings.HasPrefix(val, "trickest://file/") {
								newPNode.Value = val
							} else {
								newPNode.Value = "trickest://file/" + val
								trueVal := true
								newPNode.UpdateFile = &trueVal
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

	workflowID := uuid.Nil
	if workflow == nil {
		projectID := uuid.Nil
		if project != nil {
			projectID = project.ID
		}
		if workflowName == "" {
			fmt.Println("Use --set-name flag when trying to create a new workflow.")
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

	uploadFilesIfNeeded(version.Data.PrimitiveNodes)
	setConnectedSplitters(version, nil)
	generateNodesCoordinates(version)
	version = createNewVersion(version)
	return version
}

func createToolWorkflow(wfName string, space *types.SpaceDetailed, project *types.Project, deleteProjectOnError bool,
	tool *types.Tool, primitiveNodes map[string]*types.PrimitiveNode, machine types.Machines) *types.WorkflowVersionDetailed {
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
			if *primitiveNode.ParamName == inputName {
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
			}{ID: "input/" + node.Name + "/" + *pNode.ParamName},
		})
	}

	wfExists := false
	workflowID := uuid.Nil
	projectID := uuid.Nil
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

	uploadFilesIfNeeded(newVersion.Data.PrimitiveNodes)
	newVersion = createNewVersion(newVersion)
	return newVersion
}

func prepareForExec(objectPath string) *types.WorkflowVersionDetailed {
	pathSplit := strings.Split(strings.Trim(objectPath, "/"), "/")
	var wfVersion *types.WorkflowVersionDetailed
	var primitiveNodes map[string]*types.PrimitiveNode
	projectCreated := false

	space, project, workflow, _ := util.ResolveObjectPath(objectPath, false, false)
	if workflow == nil {
		space, project, workflow, _ = util.ResolveObjectURL(util.URL)
	}

	if workflow != nil && newWorkflowName == "" {
		// Executing an existing workflow
		wfVersion = GetLatestWorkflowVersion(workflow.ID)
		if configFile == "" {
			executionMachines = wfVersion.MaxMachines
		} else {
			update, updatedWfVersion, newPrimitiveNodes := readConfig(configFile, wfVersion, nil)
			if update {
				uploadFilesIfNeeded(newPrimitiveNodes)
				wfVersion = createNewVersion(updatedWfVersion)
				return wfVersion
			}
		}
	} else {
		// Executing from library
		wfName := pathSplit[len(pathSplit)-1]
		libraryWorkflows := util.GetWorkflows(uuid.Nil, uuid.Nil, wfName, true)
		if libraryWorkflows != nil && len(libraryWorkflows) > 0 {
			// Executing from library
			for _, wf := range libraryWorkflows {
				if strings.ToLower(wf.Name) == strings.ToLower(wfName) {
					if project == nil && createProject {
						projectName := util.ProjectName
						if projectName == "" {
							projectName = wfName
						}
						project = create.CreateProjectIfNotExists(space, projectName)
						projectCreated = true
					}

					if newWorkflowName == "" {
						newWorkflowName = wf.Name
					}
					copyDestination := space.Name
					if project != nil {
						copyDestination += "/" + project.Name
					}
					copyDestination += "/" + newWorkflowName
					fmt.Println("Copying " + wf.Name + " from the library to " + copyDestination)
					projID := uuid.Nil
					if project != nil {
						projID = project.ID
					}
					newWorkflowID := copyWorkflow(space.ID, projID, wf.ID)
					if newWorkflowID == uuid.Nil {
						fmt.Println("Couldn't copy workflow from the library!")
						os.Exit(0)
					}

					newWorkflow := util.GetWorkflowByID(newWorkflowID)
					if newWorkflow.Name != newWorkflowName {
						newWorkflow.Name = newWorkflowName
						updateWorkflow(newWorkflow, projectCreated)
					}

					copiedWfVersion := GetLatestWorkflowVersion(newWorkflow.ID)
					if copiedWfVersion == nil {
						fmt.Println("No workflow version found for " + newWorkflow.Name)
						os.Exit(0)
					}
					update := false
					var updatedWfVersion *types.WorkflowVersionDetailed
					if configFile == "" {
						executionMachines = copiedWfVersion.MaxMachines
					} else {
						update, updatedWfVersion, primitiveNodes = readConfig(configFile, copiedWfVersion, nil)
					}

					if update {
						if updatedWfVersion == nil {
							fmt.Println("Sorry, couldn't update workflow!")
							os.Exit(0)
						}
						uploadFilesIfNeeded(primitiveNodes)
						updatedWfVersion.WorkflowInfo = newWorkflow.ID
						wfVersion = createNewVersion(updatedWfVersion)
					} else {
						copiedWfVersion.WorkflowInfo = newWorkflow.ID
						if len(copiedWfVersion.Data.PrimitiveNodes) > 0 {
							for _, pNode := range copiedWfVersion.Data.PrimitiveNodes {
								pNode.Coordinates.X += 0.00001
								break
							}
						} else if len(copiedWfVersion.Data.Nodes) > 0 {
							for _, node := range copiedWfVersion.Data.Nodes {
								node.Meta.Coordinates.X += 0.00001
								break
							}
						} else {
							fmt.Println("No nodes found in workflow version!")
							os.Exit(0)
						}
						wfVersion = createNewVersion(copiedWfVersion)
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
			fmt.Println("Couldn't find a workflow or tool named " + wfName + " in the library!")
			fmt.Println("Use \"trickest library list\" to see all available workflows and tools, " +
				"or search the library using \"trickest library search <name/description>\"")
			os.Exit(0)
		}
		_, _, primitiveNodes = readConfig(configFile, nil, &tools[0])

		if project == nil && createProject {
			projectName := util.ProjectName
			if projectName == "" {
				projectName = wfName
			}
			project = create.CreateProjectIfNotExists(space, tools[0].Name)
			projectCreated = true
		}
		if newWorkflowName == "" {
			newWorkflowName = wfName
		}
		wfVersion = createToolWorkflow(newWorkflowName, space, project, projectCreated, &tools[0], primitiveNodes, executionMachines)

		return wfVersion
	}

	wfVersion = createNewVersion(wfVersion)
	return wfVersion
}
