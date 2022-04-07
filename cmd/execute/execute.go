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
)

// ExecuteCmd represents the execute command
var ExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "This command executes a workflow",
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

		allNodes, roots = CreateTrees(version)
		createRun(version.ID, watch, &executionMachines)
	},
}

func init() {
	ExecuteCmd.Flags().StringVar(&newWorkflowName, "name", "", "New workflow name (used when creating tool workflows or copying store workflows)")
	ExecuteCmd.Flags().StringVar(&configFile, "config", "", "YAML file for run configuration")
	ExecuteCmd.Flags().BoolVar(&watch, "watch", false, "Watch the execution running")
	ExecuteCmd.Flags().BoolVar(&showParams, "show-params", false, "Show parameters in the workflow tree")
	ExecuteCmd.Flags().StringVar(&workflowYAML, "file", "", "Workflow YAML file to execute")
}

func getToolScriptORsplitterFromYAMLNode(node types.WorkflowYAMLNode) (*types.Tool, *types.Script, *types.Splitter) {
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

func nodeExists(nodes []types.WorkflowYAMLNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func readWorkflowYAMLandCreateVersion(fileName string, workflowName string, path string) *types.WorkflowVersionDetailed {
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

	space, project, workflow, _ := list.ResolveObjectPath(path, true)
	if space == nil {
		fmt.Println("Space " + strings.Split(path, "/")[0] + " doesn't exist!")
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
		tool, script, splitter := getToolScriptORsplitterFromYAMLNode(node)

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

			inputs, ok := node.Inputs.([]interface{})
			if !ok {
				fmt.Println("Invalid inputs format: ")
				fmt.Println(node)
				os.Exit(0)
			}
			for _, value := range inputs {
				newPNode := types.PrimitiveNode{
					Coordinates: struct {
						X float64 `json:"x"`
						Y float64 `json:"y"`
					}{0, 0},
				}
				switch val := value.(type) {
				case string:
					if strings.Contains(val, ".") {
						if strings.HasPrefix(val, "http") {
							if strings.HasSuffix(val, ".git") {
								newPNode.Type = "FOLDER"
								gitInputCnt++
								newPNode.Name = "git-input-" + strconv.Itoa(gitInputCnt)
							} else {
								newPNode.Type = "FILE"
								httpInputCnt++
								newPNode.Name = "http-input-" + strconv.Itoa(httpInputCnt)
							}
							newPNode.Value = val
						} else {
							newPNode.Type = "FILE"
							httpInputCnt++
							newPNode.Name = "http-input-" + strconv.Itoa(httpInputCnt)
							newPNode.Value = "trickest://file/" + val
						}
						newPNode.TypeName = "URL"
						pathSplit := strings.Split(val, "/")
						newPNode.Label = pathSplit[len(pathSplit)-1]
					} else {
						if !nodeExists(wfNodes, val) {
							if _, err = os.Stat(val); errors.Is(err, os.ErrNotExist) {
								fmt.Println("A node with the given ID (" + val + ") doesn't exists in the workflow yaml!")
								fmt.Println("A file named " + val + " doesn't exist!")
								os.Exit(0)
							} else {
								newPNode.Type = "FILE"
								httpInputCnt++
								newPNode.Name = "http-input-" + strconv.Itoa(httpInputCnt)
								newPNode.Value = "trickest://file/" + val
								newPNode.TypeName = "URL"
							}
						} else {
							break //input from an existing node
							//todo folder inputs for scripts (from node outputs)
						}
					}
				default:
					fmt.Println(node)
					fmt.Println("Unknown type for script input! Use node ID, file or folder.")
					os.Exit(0)
				}
				connection := types.Connection{
					Source: struct {
						ID string `json:"id"`
					}{},
					Destination: struct {
						ID string `json:"id"`
					}{},
				}
				inputName := "file/"
				inputValue := "in/"
				if newPNode.Name != "" {
					connection.Source.ID = "output/" + newPNode.Name + "/output"
					inputName += newPNode.Name
					inputValue += newPNode.Name + "/" + newPNode.Label
					primitiveNodes[newPNode.Name] = &newPNode
				} else {
					sourceNodeID := value.(string)
					connection.Source.ID = "output/" + sourceNodeID + "/file"
					inputName += sourceNodeID
					inputValue += sourceNodeID + "/output.txt"
				}
				connection.Destination.ID = "input/" + node.ID + "/" + inputName
				connections = append(connections, connection)
				in, exists := newNode.Inputs[inputName]
				if exists {
					fmt.Println("Input with the same name already exists!")
					fmt.Println("Name: " + inputName)
					fmt.Println("Values: ")
					fmt.Println(value)
					fmt.Println(in.Value)
					os.Exit(0)
				}
				newNode.Inputs[inputName] = &types.NodeInput{
					Type:  "FILE",
					Order: 0,
					Value: inputValue,
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
								newNode.WorkerConnected = &val
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
						pathSplit := strings.Split(val, "/")
						newPNode.Label = pathSplit[len(pathSplit)-1]
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
								newNode.WorkerConnected = &val
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
							newNode.WorkerConnected = &val
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
					newNode.Inputs[name].Value = "in/" + newPNode.Name + "/" + newPNode.Label
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
			newNode.Outputs.Output = &splitter.Outputs.Output
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
						Value: "in/" + val + "output.txt",
					}
					newNode.Outputs.Output = &splitter.Outputs.Output
				default:
					fmt.Println(node)
					fmt.Println("Unknown type for file splitter! Use node ID instead.")
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
	version = createNewVersion(version)
	executionMachines = version.MaxMachines
	return version
}

func WatchRun(runID string, nodesToDownload map[string]download.NodeInfo, timestampOnly bool, machines *types.Bees) {
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
			out += fmt.Sprintf(fmtStr, "Started:", run.StartedDate.In(time.Local).Format(time.RFC1123)+
				" ("+time.Since(run.StartedDate).Round(time.Second).String()+" ago)")
		}
		if run.Finished {
			out += fmt.Sprintf(fmtStr, "Finished:", run.CompletedDate.In(time.Local).Format(time.RFC1123)+
				" ("+time.Since(run.CompletedDate).Round(time.Second).String()+" ago)")
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

		trees := PrintTrees(roots, &allNodes, showParams, true)
		out += "\n" + trees
		_, _ = fmt.Fprintln(writer, out)
		_ = writer.Flush()

		if timestampOnly {
			return
		}

		if run.Status == "COMPLETED" || run.Status == "STOPPED" || run.Status == "FAILED" {
			if len(nodesToDownload) > 0 {
				path := run.SpaceName
				if run.ProjectName != "" {
					path += "/" + run.ProjectName
				}
				path += "/" + run.WorkflowName
				download.DownloadRunOutput(run, nodesToDownload, nil, path)
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
						v = getNodeNameFromConnectionID(v)
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

func CreateTrees(wfVersion *types.WorkflowVersionDetailed) (map[string]*types.TreeNode, []*types.TreeNode) {
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
		}
	}

	for node := range wfVersion.Data.Nodes {
		for _, connection := range wfVersion.Data.Connections {
			if node == getNodeNameFromConnectionID(connection.Destination.ID) {
				child := getNodeNameFromConnectionID(connection.Source.ID)
				if childNode, exists := allNodes[child]; exists {
					childNode.HasParent = true
					allNodes[node].Children = append(allNodes[node].Children, childNode)
				}
			}
		}
	}

	for node := range wfVersion.Data.Nodes {
		if !allNodes[node].HasParent {
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
		if pNode, exists := primitiveNodes[inputName]; exists {
			inputs[inputName].Value = pNode.Value
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
			}{ID: "input/" + node.Name + "/" + pNode.Name},
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

func prepareForExec(path string) *types.WorkflowVersionDetailed {
	pathSplit := strings.Split(strings.Trim(path, "/"), "/")
	var wfVersion *types.WorkflowVersionDetailed
	var primitiveNodes map[string]*types.PrimitiveNode
	projectCreated := false

	space, project, workflow, _ := list.ResolveObjectPath(path, false)
	if space == nil {
		os.Exit(0)
	}
	if workflow == nil {
		wfName := pathSplit[len(pathSplit)-1]
		storeWorkflows := list.GetWorkflows("", true, wfName)
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
			update, updatedWfVersion, _ := readConfig(configFile, wfVersion, nil)
			if update {
				for _, pNode := range primitiveNodes {
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
		inputsList, isList := inputs.([]interface{})
		if !isList {
			processInvalidInputStructure()
		}
		if tool != nil && len(inputsList) == 0 {
			fmt.Println("You must specify input parameters when creating a tool workflow!")
			os.Exit(0)
		}
		for _, in := range inputsList {
			input, structure := in.(map[string]interface{})
			if !structure {
				processInvalidInputStructure()
			}

			for param, paramValue := range input {
				newPNode := types.PrimitiveNode{Name: param, Value: paramValue}
				if !strings.Contains(newPNode.Name, ".") {
					if tool != nil {
						newPNode.Name = tool.Name + "." + newPNode.Name
					} else if wfVersion != nil && len(wfVersion.Data.Nodes) == 1 {
						for n := range wfVersion.Data.Nodes {
							newPNode.Name = n + "." + newPNode.Name
						}
					}
				}
				nameSplit := strings.Split(newPNode.Name, ".")
				if len(nameSplit) != 2 {
					fmt.Println("Invalid input parameter: " + param)
					os.Exit(0)
				}
				paramName := nameSplit[1]
				nodeName := nameSplit[0]

				nodeNameSplit := strings.Split(nodeName, "-")
				if _, e := strconv.Atoi(nodeNameSplit[len(nodeNameSplit)-1]); e != nil {
					if wfVersion == nil {
						newPNode.Name = paramName
					} else {
						nodeName += findStartingNodeSuffix(wfVersion)
					}
				}

				switch val := newPNode.Value.(type) {
				case string:
					newPNode.Type = "STRING"
					newPNode.TypeName = "STRING"
				case int:
					newPNode.Type = "STRING"
					newPNode.TypeName = "STRING"
					newPNode.Value = strconv.Itoa(val)
				case bool:
					newPNode.Type = "BOOLEAN"
					newPNode.TypeName = "BOOLEAN"
					newPNode.Value = val
				case map[string]interface{}:
					if fName, isFile := val["file"]; isFile {
						newPNode.Type = "FILE"
						newPNode.Value = "trickest://file/" + fName.(string)
						pathSplit := strings.Split(fName.(string), "/")
						newPNode.Label = pathSplit[len(pathSplit)-1]
					} else if url, isURL := val["url"]; isURL {
						newPNode.Value = url.(string)
						if strings.HasSuffix(url.(string), ".git") {
							newPNode.Type = "FOLDER"
						} else {
							newPNode.Type = "FILE"
						}
						pathSplit := strings.Split(fName.(string), "/")
						newPNode.Label = pathSplit[len(pathSplit)-1]
					} else {
						newPNode.Type = "OBJECT"
					}
					newPNode.TypeName = "URL"
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
					node, nodeExists := wfVersion.Data.Nodes[nodeName]
					if !nodeExists {
						fmt.Println("Node doesn't exist: " + nodeName)
						os.Exit(0)
					}

					oldParam, paramExists := node.Inputs[paramName]
					paramExists = paramExists && oldParam.Value != nil
					if !paramExists {
						fmt.Println("Parameter " + paramName + " doesn't exist for node " + nodeName)
						os.Exit(0)
					}

					connectionFound := false
					for _, connection := range wfVersion.Data.Connections {
						if strings.HasSuffix(connection.Destination.ID, nodeName+"/"+paramName) {
							connectionFound = true
							primitiveNodeName := getNodeNameFromConnectionID(connection.Source.ID)
							primitiveNode, pNodeExists := wfVersion.Data.PrimitiveNodes[primitiveNodeName]
							if !pNodeExists {
								fmt.Println(primitiveNodeName + " is not a primitive node output!")
								os.Exit(0)
							}

							savedPNode, alreadyExists := newPrimitiveNodes[primitiveNode.Name]
							if alreadyExists && (savedPNode.Value != newPNode.Value) {
								processDifferentParamsForASinglePNode(*savedPNode, newPNode)
							}

							newPrimitiveNodes[primitiveNode.Name] = &newPNode

							if oldParam.Value != newPNode.Value {
								if newPNode.Type != primitiveNode.Type {
									processInvalidInputType(newPNode, *primitiveNode)
								}
								primitiveNode.Value = newPNode.Value
								primitiveNode.Label = newPNode.Label
								oldParam.Value = newPNode.Value
								wfVersion.Name = nil
								updateNeeded = true
							}
							break
						}
					}
					if !connectionFound {
						fmt.Println(newPNode.Name + " is not connected to any input!")
						os.Exit(0)
					}
				} else {
					toolInput, paramExists := tool.Inputs[newPNode.Name]
					if !paramExists {
						fmt.Println("Input parameter " + newPNode.Name + " doesn't exist for tool named " + tool.Name + "!")
						os.Exit(0)
					}
					if strings.ToLower(newPNode.Type) != strings.ToLower(toolInput.Type) {
						fmt.Println("Input parameter " + tool.Name + "." + newPNode.Name + " should be of type " + tool.Type + " instead of " + newPNode.Type + "!")
						os.Exit(0)
					}
					newPrimitiveNodes[newPNode.Name] = &newPNode
				}
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
		machinesList, ok := machines.([]interface{})
		if !ok {
			fmt.Println("Invalid machines format! Use a list of machine dictionaries (small, medium, large) instead.")
			os.Exit(0)
		}

		for _, m := range machinesList {
			machine, structure := m.(map[string]interface{})
			if !structure {
				processInvalidMachineStructure()
			}

			small, isSmall := machine["small"]
			medium, isMedium := machine["medium"]
			large, isLarge := machine["large"]

			if !isSmall && !isMedium && !isLarge {
				fmt.Print("Unrecognized machine: ")
				fmt.Println(machine)
				os.Exit(0)
			}

			var val interface{}
			if isSmall {
				val = small
			} else if isMedium {
				val = medium
			} else if isLarge {
				val = large
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
				if strings.ToLower(value) == "max" || strings.ToLower(value) == "maximum" {
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
			return &types.Bees{Large: &oneMachine}
		} else {
			return maximumMachines
		}
	}

	return execMachines
}
