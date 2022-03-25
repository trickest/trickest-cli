package execute

import (
	"bytes"
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

		version := prepareForExec(path)
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
		Outputs:       tool.Outputs,
		OutputCommand: tool.OutputCommand,
	}
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
			Command:     toolInput.Command,
			Description: toolInput.Description,
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

	space, project, workflow, _ := list.ResolveObjectPath(path)
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
					} else if url, isURL := val["url"]; isURL {
						newPNode.Value = url.(string)
						if strings.HasSuffix(url.(string), ".git") {
							newPNode.Type = "FOLDER"
						} else {
							newPNode.Type = "FILE"
						}
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
