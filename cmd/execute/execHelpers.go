package execute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"io"
	"io/ioutil"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"trickest-cli/cmd/delete"
	"trickest-cli/cmd/download"
	"trickest-cli/types"
	"trickest-cli/util"
)

func getSplitter() *types.Splitter {
	client := &http.Client{}
	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/store/splitter/", nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Error: Couldn't get splitter.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read splitter response.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(resp)
	}

	var splitters types.SplitterResponse
	err = json.Unmarshal(bodyBytes, &splitters)
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
	urlReq := util.Cfg.BaseUrl + "v1/store/script/"
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

	client := &http.Client{}
	req, err := http.NewRequest("GET", urlReq, nil)
	req.Header.Add("Authorization", "Token "+util.Cfg.User.Token)
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Error: Couldn't get scripts.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read scripts response.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(resp)
	}

	var scripts types.Scripts
	err = json.Unmarshal(bodyBytes, &scripts)
	if err != nil {
		fmt.Println("Error unmarshalling scripts response!")
		return nil
	}

	return scripts.Results
}

func createRun(versionID string, watch bool, machines *types.Bees) {
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
		util.ProcessUnexpectedResponse(resp)
	}

	var createRunResp types.CreateRunResponse
	err = json.Unmarshal(bodyBytes, &createRunResp)
	if err != nil {
		fmt.Println("Error unmarshalling create run response!")
		os.Exit(0)
	}

	if watch {
		WatchRun(createRunResp.ID, nodesToDownload, false, &executionMachines, showParams)
	} else {
		availableBees := GetAvailableMachines()
		fmt.Println("Run successfully created! ID: " + createRunResp.ID)
		fmt.Print("Machines:\n" + FormatMachines(machines, false))
		fmt.Print("\nAvailable:\n" + FormatMachines(&availableBees, false))
	}
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
		util.ProcessUnexpectedResponse(resp)
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

	if resp.StatusCode != http.StatusCreated {
		util.ProcessUnexpectedResponse(resp)
	}

	fmt.Println(filepath.Base(file.Name()) + " successfully uploaded!\n")
	return filepath.Base(file.Name())
}

func GetLatestWorkflowVersion(workflow *types.Workflow) *types.WorkflowVersionDetailed {
	if workflow == nil {
		fmt.Println("No workflow provided, couldn't find the latest version!")
		os.Exit(0)
	}

	if workflow.VersionCount == nil || *workflow.VersionCount == 0 {
		fmt.Println("This workflow has no versions!")
		os.Exit(0)
	}

	versions := getWorkflowVersions(workflow.ID, 1)

	if versions == nil || len(versions) == 0 {
		fmt.Println("Couldn't find any versions of the workflow: " + workflow.Name)
		os.Exit(0)
	}

	latestVersion := download.GetWorkflowVersionByID(versions[0].ID)

	return latestVersion
}

func getWorkflowVersions(workflowID string, pageSize int) []types.WorkflowVersion {
	if workflowID == "" {
		fmt.Println("No workflow ID provided, couldn't find versions!")
		os.Exit(0)
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
		os.Exit(0)
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read workflow versions.")
		os.Exit(0)
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(resp)
	}

	var versions types.WorkflowVersions
	err = json.Unmarshal(bodyBytes, &versions)
	if err != nil {
		fmt.Println("Error unmarshalling workflow versions response!")
		os.Exit(0)
	}

	return versions.Results
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
		util.ProcessUnexpectedResponse(resp)
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

func updateWorkflow(workflow *types.Workflow, deleteProjectOnError bool) {
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
		if deleteProjectOnError {
			delete.DeleteProject(workflow.ProjectInfo)
		}
		util.ProcessUnexpectedResponse(resp)
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

func processMaxMachinesOverflow(maximumMachines *types.Bees) {
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
	fmt.Println(newPNode.Name + " should be of type " + printType + " instead of " +
		strings.ToLower(newPNode.Type) + "!")
	os.Exit(0)
}

func GetAvailableMachines() types.Bees {
	hiveInfo := util.GetHiveInfo()
	availableBees := types.Bees{}
	for _, bee := range hiveInfo.Bees {
		if bee.Name == "small" {
			available := bee.Total - bee.Running
			availableBees.Small = &available
		}
		if bee.Name == "medium" {
			available := bee.Total - bee.Running
			availableBees.Medium = &available
		}
		if bee.Name == "large" {
			available := bee.Total - bee.Running
			availableBees.Large = &available
		}
	}
	return availableBees
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
		util.ProcessUnexpectedResponse(resp)
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
		util.ProcessUnexpectedResponse(resp)
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
		util.ProcessUnexpectedResponse(resp)
	}
}

func FormatMachines(machines *types.Bees, inline bool) string {
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
