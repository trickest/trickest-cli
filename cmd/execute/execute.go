package execute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gosuri/uilive"
	"github.com/spf13/cobra"
	"github.com/xlab/treeprint"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	spaceName         string
	workflowName      string
	configFile        string
	watch             bool
	showParams        bool
	executionMachines types.Bees
	maxMachines       types.Bees
	availableMachines types.Bees
	hive              *types.Hive
	nodesToDownload   = make(map[string]download.NodeInfo, 0)
	workflow          *types.Workflow
	version           *types.WorkflowVersionDetailed
	allNodes          map[string]*types.TreeNode
	roots             []*types.TreeNode
)

// ExecuteCmd represents the execute command
var ExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "This command executes a workflow",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Workflow name or path must be specified!")
			return
		}

		hive = util.GetHiveInfo()
		if hive == nil {
			return
		}

		workflow = prepareForExec(args[0])
		version = getLatestWorkflowVersion(workflow)
		if version == nil {
			fmt.Println("Couldn't get the latest workflow version!")
			os.Exit(0)
		}
		maxMachines = version.MaxMachines
		getAvailableMachines()
		if configFile != "" {
			update := readConfig(configFile)
			if update {
				version = createNewVersion(version)
			}
		} else {
			executionMachines = maxMachines
		}

		allNodes, roots = createTrees(version)
		createRun(version.ID, watch)
	},
}

func init() {
	ExecuteCmd.Flags().StringVar(&spaceName, "space", "Playground", "Space name")
	ExecuteCmd.Flags().StringVar(&workflowName, "name", "", "Workflow name")
	ExecuteCmd.Flags().StringVar(&configFile, "config", "", "YAML file for run configuration")
	ExecuteCmd.Flags().BoolVar(&watch, "watch", false, "Watch the execution running")
	ExecuteCmd.Flags().BoolVar(&showParams, "show-params", false, "Show parameters in the workflow tree")
}

func watchRun(runID string, nodesToDownload map[string]download.NodeInfo) {
	const fmtStr = "%-12s %v\n"
	writer := uilive.New()
	writer.Start()
	defer writer.Stop()

	mutex := &sync.Mutex{}

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

	for {
		mutex.Lock()
		run := GetRunByID(runID)
		if run == nil {
			mutex.Unlock()
			break
		}

		out := ""
		out += fmt.Sprintf(fmtStr, "Name:", workflow.Name)
		out += fmt.Sprintf(fmtStr, "Status:", strings.ToLower(run.Status))
		out += fmt.Sprintf(fmtStr, "Machines:", formatMachines(&executionMachines, true))
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

		trees := printTrees(roots, &allNodes)
		out += "\n" + trees
		_, _ = fmt.Fprintln(writer, out)
		_ = writer.Flush()

		if run.Status == "COMPLETED" || run.Status == "STOPPED" || run.Status == "FAILED" {
			if len(nodesToDownload) > 0 {
				path := spaceName
				if workflow.ProjectName != "" {
					path += "/" + workflow.ProjectName
				}
				path += "/" + workflow.Name
				download.DownloadRunOutput(run, nodesToDownload, version, path)
			}
			mutex.Unlock()
			return
		}
		mutex.Unlock()
	}
}

func createRun(versionID string, watch bool) {
	run := types.CreateRun{
		VersionID: versionID,
		HiveInfo:  hive.ID,
		Bees:      executionMachines,
	}

	buf := new(bytes.Buffer)

	err := json.NewEncoder(buf).Encode(&run)
	if err != nil {
		fmt.Println("Error encoding create run request!")
		os.Exit(0)
	}

	bodyData := bytes.NewReader(buf.Bytes())

	client := &http.Client{}
	req, err := http.NewRequest("POST", util.Cfg.BaseUrl+"v1/run/", bodyData)
	req.Header.Add("Authorization", "Token "+util.Cfg.User.Token)
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't create run!")
		os.Exit(0)
	}

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read response body!")
		os.Exit(0)
	}

	if resp.StatusCode != http.StatusCreated {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var createRunResp types.CreateRunResponse
	err = json.Unmarshal(bodyBytes, &createRunResp)
	if err != nil {
		fmt.Println("Error unmarshalling create run response!")
		os.Exit(0)
	}

	if watch {
		watchRun(createRunResp.ID, nodesToDownload)
	} else {
		fmt.Println("Run successfully created! ID: " + createRunResp.ID)
		fmt.Print("Machines:\n " + formatMachines(&executionMachines, false))
	}
}

func printTrees(roots []*types.TreeNode, allNodes *map[string]*types.TreeNode) string {
	trees := ""
	for _, root := range roots {
		tree := printTree(root, nil, allNodes)

		for _, node := range *allNodes {
			node.Printed = false
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

func printTree(node *types.TreeNode, branch *treeprint.Tree, allNodes *map[string]*types.TreeNode) string {
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

	if showParams {
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
			printTree(child, branch, allNodes)
		}
	}

	(*allNodes)[node.NodeName].Printed = true

	return (*branch).String()
}

func createTrees(wfVersion *types.WorkflowVersionDetailed) (map[string]*types.TreeNode, []*types.TreeNode) {
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

func getNodeNameFromConnectionID(id string) string {
	idSplit := strings.Split(id, "/")
	if len(idSplit) < 3 {
		fmt.Println("Invalid source/destination ID!")
		os.Exit(0)
	}

	return idSplit[1]
}

func readConfig(fileName string) bool {
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

	if machines, exists := config["machines"]; exists && machines != nil {
		machinesList := machines.([]interface{})

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
					numberOfMachines = &value
				}
			case string:
				if strings.ToLower(value) == "max" || strings.ToLower(value) == "maximum" {
					if isSmall {
						numberOfMachines = maxMachines.Small
					} else if isMedium {
						numberOfMachines = maxMachines.Medium
					} else if isLarge {
						numberOfMachines = maxMachines.Large
					}
				} else {
					processInvalidMachineString(value)
				}
			default:
				processInvalidMachineType(value)
			}

			if isSmall {
				executionMachines.Small = numberOfMachines
			} else if isMedium {
				executionMachines.Medium = numberOfMachines
			} else if isLarge {
				executionMachines.Large = numberOfMachines
			}

		}

		if (executionMachines.Small != nil && *executionMachines.Small < 0) ||
			(executionMachines.Medium != nil && *executionMachines.Medium < 0) ||
			(executionMachines.Large != nil && *executionMachines.Large < 0) {
			fmt.Println("Number of machines cannot be negative!")
			os.Exit(0)
		}

		maxMachinesOverflow := false
		if executionMachines.Small != nil {
			if maxMachines.Small == nil || (*executionMachines.Small > *maxMachines.Small) {
				maxMachinesOverflow = true
			}
		}
		if executionMachines.Medium != nil {
			if maxMachines.Medium == nil || (*executionMachines.Medium > *maxMachines.Medium) {
				maxMachinesOverflow = true
			}
		}
		if executionMachines.Large != nil {
			if maxMachines.Large == nil || (*executionMachines.Large > *maxMachines.Large) {
				maxMachinesOverflow = true
			}
		}

		if maxMachinesOverflow {
			processMaxMachinesOverflow()
		}
	} else {
		executionMachines = maxMachines
	}

	if outputs, exists := config["outputs"]; exists && outputs != nil {
		outputsList := outputs.([]interface{})

		for _, node := range outputsList {
			nodeName, ok := node.(string)
			if ok {
				nodesToDownload[nodeName] = download.NodeInfo{ToFetch: true, Found: false}
			} else {
				fmt.Print("Invalid output node name: ")
				fmt.Println(node)
			}
		}
	}

	updateNeeded := false
	newPrimitiveNodes := make(map[string]types.PrimitiveNode, 0)
	if inputs, exists := config["inputs"]; exists && inputs != nil {
		inputsList := inputs.([]interface{})
		for _, in := range inputsList {
			input, structure := in.(map[string]interface{})
			if !structure {
				processInvalidInputStructure()
			}

			for param, paramValue := range input {
				newPNode := types.PrimitiveNode{Name: param, Value: paramValue}
				nameSplit := strings.Split(newPNode.Name, ".")
				if len(nameSplit) != 2 {
					fmt.Println("Invalid input parameter: " + newPNode.Name)
					os.Exit(0)
				}
				paramName := nameSplit[1]
				nodeName := nameSplit[0]

				nodeNameSplit := strings.Split(nodeName, "-")
				if _, e := strconv.Atoi(nodeNameSplit[len(nodeNameSplit)-1]); e != nil {
					suffix := findStartingNodeSuffix(version)
					nodeName += suffix
				}

				node, nodeExists := version.Data.Nodes[nodeName]
				if !nodeExists {
					fmt.Println("Node doesn't exist: " + nodeName)
					os.Exit(0)
				}

				switch val := newPNode.Value.(type) {
				case string:
					newPNode.Type = "STRING"
				case int:
					newPNode.Type = "STRING"
					newPNode.Value = strconv.Itoa(val)
				case bool:
					newPNode.Type = "BOOLEAN"
					newPNode.Value = val
				case map[string]interface{}:
					if fName, isFile := val["file"]; isFile {
						newPNode.Type = "FILE"
						newPNode.Value = "trickest://file/" + uploadFile(fName.(string))
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
				default:
					newPNode.Type = "UNKNOWN"
				}

				oldParam, paramExists := node.Inputs[paramName]
				paramExists = paramExists && oldParam.Value != nil
				if !paramExists {
					fmt.Println("Parameter " + paramName + " doesn't exist for node " + nodeName)
					os.Exit(0)
				}

				connectionFound := false
				for _, connection := range version.Data.Connections {
					if strings.HasSuffix(connection.Destination.ID, nodeName+"/"+paramName) {
						connectionFound = true
						primitiveNodeName := getNodeNameFromConnectionID(connection.Source.ID)
						primitiveNode, pNodeExists := version.Data.PrimitiveNodes[primitiveNodeName]
						if !pNodeExists {
							fmt.Println(primitiveNodeName + " is not a primitive node output!")
							os.Exit(0)
						}

						savedPNode, alreadyExists := newPrimitiveNodes[primitiveNode.Name]
						if alreadyExists && (savedPNode.Value != newPNode.Value) {
							processDifferentParamsForASinglePNode(savedPNode, newPNode)
						}

						newPrimitiveNodes[primitiveNode.Name] = newPNode

						if oldParam.Value != newPNode.Value {
							if newPNode.Type != primitiveNode.Type {
								processInvalidInputType(newPNode, *primitiveNode)
							}

							primitiveNode.Value = newPNode.Value
							oldParam.Value = newPNode.Value
							if newPNode.Type == "BOOLEAN" {
								boolValue := newPNode.Value.(bool)
								primitiveNode.Label = strconv.FormatBool(boolValue)
							} else {
								primitiveNode.Label = newPNode.Value.(string)
							}
							version.Name = nil
							updateNeeded = true
						}
						break
					}
				}
				if !connectionFound {
					fmt.Println(newPNode.Name + " is not connected to any input!")
					os.Exit(0)
				}
			}
		}
	}

	return updateNeeded
}

func createNewVersion(version *types.WorkflowVersionDetailed) *types.WorkflowVersionDetailed {
	buf := new(bytes.Buffer)

	err := json.NewEncoder(buf).Encode(version)
	if err != nil {
		fmt.Println("Error encoding create version request!")
		os.Exit(0)
	}

	bodyData := bytes.NewReader(buf.Bytes())

	client := &http.Client{}
	urlReq := util.Cfg.BaseUrl + "v1/store/workflow-version/"
	req, err := http.NewRequest("POST", urlReq, bodyData)
	req.Header.Add("Authorization", "Token "+util.Cfg.User.Token)
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get create version response.")
		os.Exit(0)
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read create version response.")
		os.Exit(0)
	}

	if resp.StatusCode != http.StatusCreated {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var newVersionInfo types.WorkflowVersion
	err = json.Unmarshal(bodyBytes, &newVersionInfo)
	if err != nil {
		fmt.Println("Error unmarshalling create version response!")
		return nil
	}

	newVersion := download.GetWorkflowVersionByID(newVersionInfo.ID)
	return newVersion
}

func uploadFile(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Couldn't open file!")
		os.Exit(0)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	defer writer.Close()

	part, err := writer.CreateFormFile("thumb", filepath.Base(file.Name()))
	if err != nil {
		fmt.Println("Error: Couldn't create form file!")
		os.Exit(0)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		fmt.Println("Error: Couldn't copy data from file!")
		os.Exit(0)
	}

	_, _ = part.Write([]byte("\n--" + writer.Boundary() + "--"))

	client := &http.Client{}
	req, err := http.NewRequest("POST", util.Cfg.BaseUrl+"v1/file/", body)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Content-Type", writer.FormDataContentType())

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't upload file!")
		os.Exit(0)
	}

	if resp.StatusCode != http.StatusCreated {
		var bodyBytes []byte
		bodyBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error: Couldn't read response body!")
			os.Exit(0)
		}

		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	fmt.Println(filepath.Base(file.Name()) + " successfully uploaded!")
	return filepath.Base(file.Name())
}

func getLatestWorkflowVersion(workflow *types.Workflow) *types.WorkflowVersionDetailed {
	if workflow == nil {
		fmt.Println("No workflow provided, couldn't find the latest version!")
		return nil
	}

	if workflow.VersionCount == nil || *workflow.VersionCount == 0 {
		fmt.Println("This workflow has no versions!")
		return nil
	}

	versions := getWorkflowVersions(workflow.ID, 1)

	if versions == nil || len(versions) == 0 {
		fmt.Println("Couldn't find any versions of the workflow: " + workflow.Name)
		return nil
	}

	latestVersion := download.GetWorkflowVersionByID(versions[0].ID)

	return latestVersion
}

func getWorkflowVersions(workflowID string, pageSize int) []types.WorkflowVersion {
	if workflowID == "" {
		fmt.Println("No workflow ID provided, couldn't find versions!")
		return nil
	}
	urlReq := util.Cfg.BaseUrl + "v1/store/workflow-version/?workflow=" + workflowID
	urlReq += "&page_size=" + strconv.Itoa(pageSize)

	client := &http.Client{}
	req, err := http.NewRequest("GET", urlReq, nil)
	req.Header.Add("Authorization", "Token "+util.Cfg.User.Token)
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get workflow versions.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read workflow versions.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var versions types.WorkflowVersions
	err = json.Unmarshal(bodyBytes, &versions)
	if err != nil {
		fmt.Println("Error unmarshalling workflow versions response!")
		return nil
	}

	return versions.Results
}

func prepareForExec(path string) *types.Workflow {
	path = spaceName + "/" + strings.Trim(path, "/")

	space, project, workflow, _ := list.ResolveObjectPath(path)
	if space == nil || project == nil {
		os.Exit(0)
	}
	if workflow == nil {
		pathSplit := strings.Split(path, "/")
		wfName := pathSplit[len(pathSplit)-1]
		storeWorkflows := list.GetWorkflows("", true, wfName)
		found := false
		if storeWorkflows != nil && len(storeWorkflows) > 0 {
			for _, wf := range storeWorkflows {
				if strings.ToLower(wf.Name) == strings.ToLower(wfName) {
					projectExists := false
					for _, proj := range space.Projects {
						if proj.Name == wfName {
							projectExists = true
							break
						}
					}
					if !projectExists {
						fmt.Println("Creating a project " + spaceName + "/" + wfName +
							" to copy the workflow from the store...")
						create.CreateProject(wfName, "", spaceName)
					}
					if workflowName == "" {
						workflowName = wf.Name
					}
					fmt.Println("Copying " + wf.Name + " from the store to " +
						space.Name + "/" + project.Name + "/" + workflowName)

					newWorkflowID := copyWorkflow(space.ID, project.ID, wf.ID)
					if newWorkflowID == "" {
						fmt.Println("Couldn't copy workflow from the store!")
						os.Exit(0)
					}
					newWorkflow := list.GetWorkflowByID(newWorkflowID)
					newWorkflow.Name = workflowName
					updateWorkflow(newWorkflow)
					workflow = newWorkflow

					found = true
					break
				}
			}
		}
		if !found {
			fmt.Println("Couldn't find a workflow named " + wfName + " in the store!")
			fmt.Println("Use \"trickest store list\" to see all available workflows, " +
				"or search the store using \"trickest store search <name/description>\"")
			os.Exit(0)
		}
	}

	return workflow
}

func copyWorkflow(destinationSpaceID string, destinationProjectID string, workflowID string) string {
	copyWf := types.CopyWorkflowRequest{
		SpaceID:   destinationSpaceID,
		ProjectID: destinationProjectID,
	}

	buf := new(bytes.Buffer)

	err := json.NewEncoder(buf).Encode(&copyWf)
	if err != nil {
		fmt.Println("Error encoding copy workflow request!")
		return ""
	}

	bodyData := bytes.NewReader(buf.Bytes())

	client := &http.Client{}
	req, err := http.NewRequest("POST", util.Cfg.BaseUrl+"v1/store/workflow/"+workflowID+"/copy/", bodyData)
	req.Header.Add("Authorization", "Token "+util.Cfg.User.Token)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't copy workflow.")
		return ""
	}

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read response body!")
		return ""
	}

	if resp.StatusCode != http.StatusCreated {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	fmt.Println("Workflow copied successfully!")
	var copyWorkflowResp types.CopyWorkflowResponse
	err = json.Unmarshal(bodyBytes, &copyWorkflowResp)
	if err != nil {
		fmt.Println("Error unmarshalling copy workflow response!")
		return ""
	}

	return copyWorkflowResp.ID
}

func updateWorkflow(workflow *types.Workflow) {
	workflow.WorkflowCategory = nil
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(workflow)
	if err != nil {
		fmt.Println("Error encoding update workflow request!")
		return
	}
	bodyData := bytes.NewReader(buf.Bytes())

	client := &http.Client{}
	req, err := http.NewRequest("PATCH", util.Cfg.BaseUrl+"v1/store/workflow/"+workflow.ID+"/", bodyData)
	req.Header.Add("Authorization", "Token "+util.Cfg.User.Token)
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't update workflow.")
		return
	}

	if resp.StatusCode != http.StatusOK {
		var bodyBytes []byte
		bodyBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error: Couldn't read response body!")
			return
		}

		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}
}

func processInvalidMachineString(s string) {
	fmt.Println("Invalid machine qualifier: " + s)
	fmt.Println("Try using max or maximum instead.")
	os.Exit(0)
}

func processInvalidMachineType(data interface{}) {
	fmt.Print("Invalid machine qualifier:")
	fmt.Println(data)
	fmt.Println("Try using a number or max/maximum instead.")
	os.Exit(0)
}

func processInvalidMachineStructure() {
	fmt.Println("Machines should be specified using the following format:")
	fmt.Println("machines:")
	fmt.Println(" 	- <machine-type>: <quantity>")
	fmt.Println("Machine type can be small, medium or large. Quantity is a number >= than 0 or max/maximum.")
	os.Exit(0)
}

func processInvalidInputStructure() {
	fmt.Println("Inputs should be specified using the following format:")
	fmt.Println("inputs:")
	fmt.Println(" 	- <tool_name>[-<number>].<parameter_name>: <value>")
	fmt.Println("<value> can be:")
	fmt.Println(" - raw value")
	fmt.Println(" - file: <file-name> (a local file that will be uploaded to the platform)")
	fmt.Println(" - url: <url> (for files and folders (git repos) stored somewhere on the web)")
	os.Exit(0)
}

func processMaxMachinesOverflow() {
	fmt.Println("Invalid number or machines!")
	fmt.Println("The maximum number of machines you can allocate for this workflow: ")
	fmt.Println(formatMachines(&maxMachines, false))
	os.Exit(0)
}

func processDifferentParamsForASinglePNode(existingPNode, newPNode types.PrimitiveNode) {
	fmt.Print("Inputs " + existingPNode.Name + " (")
	fmt.Print(existingPNode.Value)
	fmt.Print(") and " + newPNode.Name + " (")
	fmt.Print(newPNode.Value)
	fmt.Println(") must have the same value!")
	os.Exit(0)
}

func processInvalidInputType(newPNode, existingPNode types.PrimitiveNode) {
	printType := strings.ToLower(existingPNode.Type)
	if printType == "string" {
		printType += " (or integer, if a number is needed)"
	}
	fmt.Println(newPNode.Name + " should be of type " + printType + " instead of " +
		strings.ToLower(newPNode.Type) + "!")
	os.Exit(0)
}

func findStartingNodeSuffix(wfVersion *types.WorkflowVersionDetailed) string {
	suffix := "-1"
	for node := range wfVersion.Data.Nodes {
		if strings.HasSuffix(node, "-0") {
			suffix = "-0"
			return suffix
		}
	}
	for pNode := range wfVersion.Data.PrimitiveNodes {
		if strings.HasSuffix(pNode, "-0") {
			suffix = "-0"
			break
		}
	}

	return suffix
}

func getAvailableMachines() {
	for _, bee := range hive.Bees {
		if bee.Name == "small" {
			available := bee.Total - bee.Running
			availableMachines.Small = &available
		}
		if bee.Name == "medium" {
			available := bee.Total - bee.Running
			availableMachines.Small = &available
		}
		if bee.Name == "large" {
			available := bee.Total - bee.Running
			availableMachines.Small = &available
		}
	}
}

func GetRunByID(id string) *types.Run {
	client := &http.Client{}

	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/run/"+id+"/", nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get run info.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Error: Couldn't read run info.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var run types.Run
	err = json.Unmarshal(bodyBytes, &run)
	if err != nil {
		fmt.Println("Error unmarshalling run response!")
		return nil
	}

	return &run
}

func GetSubJobs(runID string) []types.SubJob {
	if runID == "" {
		fmt.Println("Couldn't list sub-jobs, no run ID parameter specified!")
		os.Exit(0)
	}
	urlReq := util.Cfg.BaseUrl + "v1/subjob/?run=" + runID
	urlReq = urlReq + "&page_size=" + strconv.Itoa(math.MaxInt)

	client := &http.Client{}
	req, err := http.NewRequest("GET", urlReq, nil)
	req.Header.Add("Authorization", "Token "+util.Cfg.User.Token)
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get sub-jobs info!")
		os.Exit(0)
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read sub-jobs info.")
		os.Exit(0)
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var subJobs types.SubJobs
	err = json.Unmarshal(bodyBytes, &subJobs)
	if err != nil {
		fmt.Println("Error unmarshalling sub-jobs response!")
		os.Exit(0)
	}

	return subJobs.Results
}

func stopRun(runID string) {
	client := &http.Client{}
	urlReq := util.Cfg.BaseUrl + "v1/run/" + runID + "/stop/"
	req, err := http.NewRequest("POST", urlReq, nil)
	req.Header.Add("Authorization", "Token "+util.Cfg.User.Token)
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't stop run.")
		os.Exit(0)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		var bodyBytes []byte
		bodyBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error: Couldn't read stop run response.")
			os.Exit(0)
		}

		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}
}

func formatMachines(machines *types.Bees, inline bool) string {
	var small, medium, large string
	if machines.Small != nil {
		small = "small: " + strconv.Itoa(*machines.Small)
	}
	if machines.Medium != nil {
		medium = "medium: " + strconv.Itoa(*machines.Medium)
	}
	if machines.Large != nil {
		large = "large: " + strconv.Itoa(*machines.Large)
	}

	out := ""
	if inline {
		if small != "" {
			out = small
		}
		if medium != "" {
			if small != "" {
				out += ", "
			}
			out += medium
		}
		if large != "" {
			if small != "" || medium != "" {
				out += ", "
			}
			out += large
		}
	} else {
		if small != "" {
			out = small + "\n"
		}
		if medium != "" {
			out += medium + "\n"
		}
		if large != "" {
			out += large + "\n"
		}
	}

	return out
}
