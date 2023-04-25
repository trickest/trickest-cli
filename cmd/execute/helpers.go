package execute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"trickest-cli/client/request"
	"trickest-cli/cmd/delete"
	"trickest-cli/cmd/list"
	"trickest-cli/cmd/output"
	"trickest-cli/types"
	"trickest-cli/util"

	"github.com/google/uuid"

	"github.com/schollz/progressbar/v3"
)

func getSplitter() *types.Splitter {
	resp := request.Trickest.Get().DoF("store/splitter/")
	if resp == nil {
		fmt.Println("Error: Couldn't get splitter.")
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var splitters types.SplitterResponse
	err := json.Unmarshal(resp.Body(), &splitters)
	if err != nil {
		fmt.Println("Error unmarshalling splitter response!")
		return nil
	}

	if splitters.Results == nil || len(splitters.Results) == 0 {
		fmt.Println("Couldn't find any splitter!")
		os.Exit(0)
	}

	return &splitters.Results[0]
}

func getScriptByName(name string) *types.Script {
	scripts := getScripts(1, "", name)
	if scripts == nil || len(scripts) == 0 {
		fmt.Println("No scripts found with the given name: " + name)
		return nil
	}
	return &scripts[0]
}

func getScripts(pageSize int, search string, name string) []types.Script {
	urlReq := "store/script/"
	if pageSize > 0 {
		urlReq = urlReq + "?page_size=" + strconv.Itoa(pageSize)
	} else {
		urlReq = urlReq + "?page_size=" + strconv.Itoa(math.MaxInt)
	}

	if search != "" {
		search = url.QueryEscape(search)
		urlReq += "&search=" + search
	}

	if name != "" {
		name = url.QueryEscape(name)
		urlReq += "&name=" + name
	}

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get scripts!")
		return nil
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var scripts types.Scripts
	err := json.Unmarshal(resp.Body(), &scripts)
	if err != nil {
		fmt.Println("Error unmarshalling scripts response!")
		return nil
	}

	return scripts.Results
}

func createRun(versionID uuid.UUID, watch bool, machines *types.Machines, outputNodes []string, outputsDir string) {
	run := types.CreateRun{
		VersionID: versionID,
		Vault:     fleet.Vault,
		Machines:  executionMachines,
	}

	data, err := json.Marshal(run)
	if err != nil {
		fmt.Println("Error encoding create run request!")
		os.Exit(0)
	}

	resp := request.Trickest.Post().Body(data).DoF("execution/")
	if resp == nil {
		fmt.Println("Error: Couldn't create run!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusCreated {
		request.ProcessUnexpectedResponse(resp)
	}

	var createRunResp types.CreateRunResponse
	err = json.Unmarshal(resp.Body(), &createRunResp)
	if err != nil {
		fmt.Println("Error unmarshalling create run response!")
		os.Exit(0)
	}

	if len(outputNodes) > 0 || downloadAllNodes {
		for _, nodeName := range outputNodes {
			nodesToDownload[nodeName] = output.NodeInfo{ToFetch: true, Found: false}
		}
		watch = true
	}
	if watch {
		WatchRun(createRunResp.ID, outputsDir, nodesToDownload, nil, false, &executionMachines, showParams)
	} else {
		availableMachines := GetAvailableMachines()
		fmt.Println("Run successfully created! ID: " + createRunResp.ID.String())
		fmt.Print("Machines:\n" + FormatMachines(*machines, false))
		fmt.Print("\nAvailable:\n" + FormatMachines(availableMachines, false))
	}
}

func createNewVersion(version *types.WorkflowVersionDetailed) *types.WorkflowVersionDetailed {
	for _, pNode := range version.Data.PrimitiveNodes {
		pNode.ParamName = nil
	}

	strippedVersion := *&types.WorkflowVersionStripped{
		Data:         version.Data,
		Description:  version.Description,
		WorkflowInfo: version.WorkflowInfo,
		Snapshot:     false,
	}

	data, err := json.Marshal(strippedVersion)
	if err != nil {
		fmt.Println("Error encoding create version request!")
		os.Exit(0)
	}

	resp := request.Trickest.Post().Body(data).DoF("store/workflow-version/")
	if resp == nil {
		fmt.Println("Error: Couldn't create version!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusCreated {
		request.ProcessUnexpectedResponse(resp)
	}

	var newVersionInfo types.WorkflowVersion
	err = json.Unmarshal(resp.Body(), &newVersionInfo)
	if err != nil {
		fmt.Println("Error unmarshalling create version response!")
		return nil
	}

	newVersion := output.GetWorkflowVersionByID(newVersionInfo.ID)
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

	fileName := filepath.Base(file.Name())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	defer writer.Close()

	part, err := writer.CreateFormFile("thumb", fileName)
	if err != nil {
		fmt.Println("Error: Couldn't create form file!")
		os.Exit(0)
	}

	fileInfo, _ := file.Stat()
	bar := progressbar.NewOptions64(
		fileInfo.Size(),
		progressbar.OptionSetDescription("Uploading "+fileName+" ..."),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() { fmt.Println() }),
	)

	_, err = io.Copy(io.MultiWriter(part, bar), file)
	if err != nil {
		fmt.Println("Couldn't upload " + fileName + "!")
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		fmt.Println("Error: Couldn't upload file!")
		os.Exit(0)
	}

	fmt.Println(filepath.Base(file.Name()) + " successfully uploaded!\n")
	return filepath.Base(file.Name())
}

func GetLatestWorkflowVersion(workflowID uuid.UUID) *types.WorkflowVersionDetailed {
	resp := request.Trickest.Get().DoF("store/workflow-version/latest/?workflow=%s", workflowID)
	if resp == nil {
		fmt.Println("Error: Couldn't get latest workflow version!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var latestVersion types.WorkflowVersionDetailed
	err := json.Unmarshal(resp.Body(), &latestVersion)
	if err != nil {
		fmt.Println("Error unmarshalling latest workflow version!")
		return nil
	}

	return &latestVersion
}

func copyWorkflow(destinationSpaceID, destinationProjectID, workflowID uuid.UUID) uuid.UUID {
	copyWf := types.CopyWorkflowRequest{
		SpaceID: destinationSpaceID,
	}

	if destinationProjectID != uuid.Nil {
		copyWf.ProjectID = &destinationProjectID
	}

	data, err := json.Marshal(copyWf)
	if err != nil {
		fmt.Println("Error marshaling copy workflow request!")
		os.Exit(0)
	}

	resp := request.Trickest.Post().Body(data).DoF("store/workflow/%s/copy/", workflowID)
	if resp == nil {
		fmt.Println("Error: Couldn't copy workflow!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusCreated {
		request.ProcessUnexpectedResponse(resp)
	}

	fmt.Println("Workflow copied successfully!")
	var copyWorkflowResp types.CopyWorkflowResponse
	err = json.Unmarshal(resp.Body(), &copyWorkflowResp)
	if err != nil {
		fmt.Println("Error unmarshalling copy workflow response!")
		return uuid.Nil
	}

	return copyWorkflowResp.ID
}

func updateWorkflow(workflow *types.Workflow, deleteProjectOnError bool) {
	workflow.WorkflowCategory = nil
	data, err := json.Marshal(workflow)
	if err != nil {
		fmt.Println("Error marshaling update workflow request!")
		os.Exit(0)
	}

	resp := request.Trickest.Patch().Body(data).DoF("store/workflow/%s/", workflow.ID)
	if resp == nil {
		fmt.Println("Error: Couldn't update workflow!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		if deleteProjectOnError {
			delete.DeleteProject(workflow.ProjectInfo)
		}
		request.ProcessUnexpectedResponse(resp)
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
	fmt.Println("   <machine-type>: <quantity>")
	fmt.Println("   ...")
	fmt.Println("\nMachine type can be small, medium or large. Quantity is a number >= than 0 or max/maximum.")
	os.Exit(0)
}

func processInvalidInputStructure() {
	fmt.Println("Inputs should be specified using the following format:")
	fmt.Println("inputs:")
	fmt.Println(" 	<tool_name>[-<number>].<parameter_name>: <value>")
	fmt.Println("<value> can be:")
	fmt.Println(" - raw value")
	fmt.Println(" - <file-name> (a local file that will be uploaded to the platform)")
	fmt.Println(" - <url> (for files and folders (git repos) stored somewhere on the web)")
	os.Exit(0)
}

func processMaxMachinesOverflow(maximumMachines types.Machines) {
	fmt.Println("Invalid number or machines!")
	fmt.Println("The maximum number of machines you can allocate for this workflow: ")
	fmt.Println(FormatMachines(maximumMachines, false))
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
	fmt.Println(*newPNode.ParamName + " should be of type " + printType + " instead of " +
		strings.ToLower(newPNode.Type) + "!")
	os.Exit(0)
}

func GetAvailableMachines() types.Machines {
	hiveInfo := util.GetFleetInfo()
	availableMachines := types.Machines{}
	for _, machine := range hiveInfo.Machines {
		if machine.Name == "small" {
			available := machine.Total - machine.Running
			availableMachines.Small = &available
		}
		if machine.Name == "medium" {
			available := machine.Total - machine.Running
			availableMachines.Medium = &available
		}
		if machine.Name == "large" {
			available := machine.Total - machine.Running
			availableMachines.Large = &available
		}
	}
	return availableMachines
}

func GetRunByID(id uuid.UUID) *types.Run {
	resp := request.Trickest.Get().DoF("execution/%s/", id)
	if resp == nil {
		fmt.Println("Error: Couldn't get run!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var run types.Run
	err := json.Unmarshal(resp.Body(), &run)
	if err != nil {
		fmt.Println("Error unmarshalling run response!")
		return nil
	}

	return &run
}

func GetSubJobs(runID uuid.UUID) []types.SubJob {
	if runID == uuid.Nil {
		fmt.Println("Couldn't list sub-jobs, no run ID parameter specified!")
		os.Exit(0)
	}
	urlReq := "subjob/?execution=" + runID.String()
	urlReq = urlReq + "&page_size=" + strconv.Itoa(math.MaxInt)

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get sub-jobs!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var subJobs types.SubJobs
	err := json.Unmarshal(resp.Body(), &subJobs)
	if err != nil {
		fmt.Println("Error unmarshalling sub-jobs response!")
		os.Exit(0)
	}

	return subJobs.Results
}

func stopRun(runID uuid.UUID) {
	resp := request.Trickest.Post().DoF("execution/%s/stop/", runID)
	if resp == nil {
		fmt.Println("Error: Couldn't stop run!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusAccepted {
		request.ProcessUnexpectedResponse(resp)
	}
}

func setMachinesToMinimum(machines *types.Machines) {
	if machines.Small != nil {
		*machines.Small = 1
	}
	if machines.Medium != nil {
		*machines.Medium = 1
	}
	if machines.Large != nil {
		*machines.Large = 1
	}
}

func FormatMachines(machines types.Machines, inline bool) string {
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
			out = " " + small + "\n "
		}
		if medium != "" {
			out += medium + "\n "
		}
		if large != "" {
			out += large + "\n"
		}
	}

	return out
}

func getNodeNameFromConnectionID(id string) string {
	idSplit := strings.Split(id, "/")
	if len(idSplit) < 3 {
		fmt.Println("Invalid source/destination ID!")
		os.Exit(0)
	}

	return idSplit[1]
}

func getFiles() []types.TrickestFile {
	urlReq := "file/?vault=" + util.GetVault().String()
	urlReq = urlReq + "&page_size=" + strconv.Itoa(math.MaxInt)

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get files!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var files types.FilesResponse
	err := json.Unmarshal(resp.Body(), &files)
	if err != nil {
		fmt.Println("Error unmarshalling sub-jobs response!")
		os.Exit(0)
	}

	return files.Results
}

func fileExistsOnPlatform(name string) bool {
	files := getFiles()
	for _, file := range files {
		if file.Name == name {
			return true
		}
	}
	return false
}

func uploadFilesIfNeeded(primitiveNodes map[string]*types.PrimitiveNode) {
	for _, pNode := range primitiveNodes {
		if pNode.Type == "FILE" && strings.HasPrefix(pNode.Value.(string), "trickest://file/") {
			fileName := strings.TrimPrefix(pNode.Value.(string), "trickest://file/")
			if pNode.UpdateFile != nil && *pNode.UpdateFile {
				pNode.Label = uploadFile(fileName)
			} else {
				if !fileExistsOnPlatform(fileName) {
					fmt.Println("\"" + fileName + "\" hasn't been uploaded yet or has been deleted from the platform." +
						" Try uploading it without the \"trickest://file/\" prefix.")
					os.Exit(0)
				}
			}
			pNode.UpdateFile = nil
		}
	}
}

func maxMachinesTypeCompatible(machines, maxMachines types.Machines) bool {
	if (machines.Small != nil && maxMachines.Small == nil) ||
		(machines.Medium != nil && maxMachines.Medium == nil) ||
		(machines.Large != nil && maxMachines.Large == nil) {
		return false
	}

	if (machines.Small == nil && maxMachines.Small != nil) ||
		(machines.Medium == nil && maxMachines.Medium != nil) ||
		(machines.Large == nil && maxMachines.Large != nil) {
		return false
	}

	return true
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
			if strings.HasPrefix(node.Name, "file-splitter") || strings.HasPrefix(node.Name, "split-to-string") {
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
					isSplitter := strings.HasPrefix(destinationNodeID, "file-splitter-") || strings.HasPrefix(destinationNodeID, "split-to-string-")
					if isSplitter ||
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
