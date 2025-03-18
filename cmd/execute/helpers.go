package execute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/cmd/delete"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"

	"github.com/google/uuid"

	"github.com/schollz/progressbar/v3"
)

func createRun(versionID, fleetID uuid.UUID, watch bool, outputNodes []string, outputsDir string, useStaticIPs bool) {

	run := types.CreateRun{
		VersionID:    versionID,
		Machines:     executionMachines,
		Fleet:        &fleetID,
		UseStaticIPs: useStaticIPs,
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
			nodesToDownload = append(nodesToDownload, nodeName)
		}
		watch = true
	}
	if watch {
		WatchRun(createRunResp.ID, outputsDir, nodesToDownload, nil, false, &executionMachines, showParams)
	} else {
		availableMachines := GetAvailableMachines(fleetName)
		fmt.Println("Run successfully created! ID: " + createRunResp.ID.String())
		fmt.Print("Machines:\n" + FormatMachines(executionMachines, false))
		fmt.Print("\nAvailable:\n" + FormatMachines(availableMachines, false))
	}
}

func createNewVersion(version *types.WorkflowVersionDetailed) *types.WorkflowVersionDetailed {
	for _, pNode := range version.Data.PrimitiveNodes {
		pNode.ParamName = nil
	}

	strippedVersion := types.WorkflowVersionStripped{
		Data:         version.Data,
		Description:  version.Description,
		WorkflowInfo: version.WorkflowInfo,
		Snapshot:     false,
		MaxMachines:  version.MaxMachines,
	}

	data, err := json.Marshal(strippedVersion)
	if err != nil {
		fmt.Println("Error encoding create version request!")
		os.Exit(0)
	}

	resp := request.Trickest.Post().Body(data).DoF("workflow-version/")
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

	fleet := util.GetFleetInfo(fleetName)
	newVersion := util.GetWorkflowVersionByID(newVersionInfo.ID, fleet.ID)
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

func GetLatestWorkflowVersion(workflowID uuid.UUID, fleetID uuid.UUID) *types.WorkflowVersionDetailed {
	resp := request.Trickest.Get().DoF("workflow-version/latest/?workflow=%s", workflowID)
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

	if fleetID != uuid.Nil {
		maxMachines, err := util.GetWorkflowVersionMaxMachines(latestVersion.ID.String(), fleetID)
		if err != nil {
			fmt.Printf("Error getting maximum machines: %v", err)
			return nil
		}
		latestVersion.MaxMachines = maxMachines

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

	resp := request.Trickest.Post().Body(data).DoF("library/workflow/%s/copy/", workflowID)
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
	workflow.WorkflowCategory = ""
	data, err := json.Marshal(workflow)
	if err != nil {
		fmt.Println("Error marshaling update workflow request!")
		os.Exit(0)
	}

	resp := request.Trickest.Patch().Body(data).DoF("workflow/%s/", workflow.ID)
	if resp == nil {
		fmt.Println("Error: Couldn't update workflow!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		if deleteProjectOnError {
			delete.DeleteProject(*workflow.ProjectInfo)
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

func GetAvailableMachines(fleetName string) types.Machines {
	hiveInfo := util.GetFleetInfo(fleetName)
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
		if machine.Name == "default" {
			available := machine.Total - machine.Running
			availableMachines.Default = &available
		}
		if machine.Name == "self_hosted" {
			available := machine.Total - machine.Running
			availableMachines.SelfHosted = &available
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

func setMachinesToMinimum(machines types.Machines) types.Machines {
	if machines.Small != nil {
		*machines.Small = 1
	}
	if machines.Medium != nil {
		*machines.Medium = 1
	}
	if machines.Large != nil {
		*machines.Large = 1
	}
	if machines.Default != nil {
		*machines.Default = 1
	}
	if machines.SelfHosted != nil {
		*machines.SelfHosted = 1
	}

	return machines
}

func FormatMachines(machines types.Machines, inline bool) string {
	smallMachines := formatSize("small", machines.Small)
	mediumMachines := formatSize("medium", machines.Medium)
	largeMachines := formatSize("large", machines.Large)
	selfHostedMachines := formatSize("self hosted", machines.SelfHosted)
	defaultMachines := formatSize("default", machines.Default)

	var out string
	if inline {
		out = joinNonEmptyValues(", ", smallMachines, mediumMachines, largeMachines, selfHostedMachines, defaultMachines)
	} else {
		out = joinNonEmptyValues("\n ", " "+smallMachines, mediumMachines, largeMachines, selfHostedMachines, defaultMachines)
	}
	return out
}

func formatSize(sizeName string, size *int) string {
	if size != nil {
		return sizeName + ": " + strconv.Itoa(*size)
	}
	return ""
}

func joinNonEmptyValues(separator string, values ...string) string {
	var nonEmptyValues []string

	for _, value := range values {
		if value != "" {
			nonEmptyValues = append(nonEmptyValues, value)
		}
	}

	result := strings.Join(nonEmptyValues, separator)
	return result
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
	return verifyMachineType(machines.Default, maxMachines.Default) &&
		verifyMachineType(machines.SelfHosted, maxMachines.SelfHosted) &&
		verifyMachineType(machines.Small, maxMachines.Small) &&
		verifyMachineType(machines.Medium, maxMachines.Medium) &&
		verifyMachineType(machines.Large, maxMachines.Large)
}

func verifyMachineType(machine, maxMachine *int) bool {
	if machine != nil && maxMachine != nil && *machine > *maxMachine {
		return false
	}

	if (machine != nil && maxMachine == nil) || (machine == nil && maxMachine != nil) {
		return false
	}

	return true
}
