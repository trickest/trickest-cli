package execute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	executionMachines types.Bees
	maxMachines       types.Bees
	availableMachines types.Bees
	hive              *types.Hive
	nodesToDownload   = make(map[string]download.NodeInfo, 0)
	workflow          *types.Workflow
	version           *types.WorkflowVersionDetailed
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
				createNewVersion(version)
			}
		}
	},
}

func init() {
	ExecuteCmd.Flags().StringVar(&spaceName, "space", "Playground", "Space name")
	ExecuteCmd.Flags().StringVar(&workflowName, "name", "", "Workflow name")
	ExecuteCmd.Flags().StringVar(&configFile, "config", "", "YAML file for run configuration")
	ExecuteCmd.Flags().BoolVar(&watch, "watch", false, "Watch the execution running")

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

	if machines, exists := config["machines"]; exists {
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
			if n, number := val.(int); number {
				if n != 0 {
					numberOfMachines = &n
				}
			} else if s, word := val.(string); word {
				if strings.ToLower(s) == "max" || strings.ToLower(s) == "maximum" {
					if isSmall {
						numberOfMachines = maxMachines.Small
					} else if isMedium {
						numberOfMachines = maxMachines.Medium
					} else if isLarge {
						numberOfMachines = maxMachines.Large
					}
				} else {
					processInvalidMachineString(s)
				}
			} else {
				processInvalidMachineType(val)
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

	if outputs, exists := config["outputs"]; exists {
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
	newPrimitiveNodes := make(map[string]struct {
		Param string
		Value interface{}
	}, 0)
	if inputs, exists := config["inputs"]; exists {
		inputsList := inputs.([]interface{})
		for _, in := range inputsList {
			input, structure := in.(map[string]interface{})
			if !structure {
				processInvalidInputStructure()
			}

			for param, paramValue := range input {
				nameSplit := strings.Split(param, ".")
				if len(nameSplit) != 2 {
					fmt.Println("Invalid input parameter: " + param)
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

				var paramType string
				if _, isStr := paramValue.(string); isStr {
					paramType = "STRING"
				} else if intVal, isInt := paramValue.(int); isInt {
					paramValue = strconv.Itoa(intVal)
				} else if boolVal, isBool := paramValue.(bool); isBool {
					paramType = "BOOLEAN"
					paramValue = boolVal
				} else if objectParam, isObject := paramValue.(map[string]interface{}); isObject {
					if fName, isFile := objectParam["file"]; isFile {
						paramType = "FILE"
						paramValue = "trickest://file/" + uploadFile(fName.(string))
					} else if url, isURL := objectParam["url"]; isURL {
						paramValue = url.(string)
						if strings.HasSuffix(url.(string), ".git") {
							paramType = "FOLDER"
						} else {
							paramType = "FILE"
						}
					} else {
						paramType = "OBJECT"
					}
				} else {
					paramType = "UNKNOWN"
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
						primitiveNodeName := strings.TrimSuffix(connection.Source.ID, "output")
						primitiveNodeName = strings.TrimPrefix(primitiveNodeName, "output")
						primitiveNodeName = strings.Trim(primitiveNodeName, "/")
						primitiveNode, pNodeExists := version.Data.PrimitiveNodes[primitiveNodeName]
						if !pNodeExists {
							fmt.Println(primitiveNodeName + " is not a primitive node output!")
							os.Exit(0)
						}

						savedPNode, alreadyExists := newPrimitiveNodes[primitiveNode.Name]
						if alreadyExists && (savedPNode.Value != paramValue) {
							fmt.Print("Inputs " + savedPNode.Param + " (")
							fmt.Print(savedPNode.Value)
							fmt.Print(") and " + param + " (")
							fmt.Print(paramValue)
							fmt.Println(") must have the same value!")
							os.Exit(0)
						}

						newPrimitiveNodes[primitiveNode.Name] = struct {
							Param string
							Value interface{}
						}{Param: param, Value: paramValue}

						if oldParam.Value != paramValue {
							if paramType != primitiveNode.Type {
								printType := strings.ToLower(primitiveNode.Type)
								if printType == "string" {
									printType += " (or integer, if a number is needed)"
								}
								fmt.Println(param + " should be of type " + printType + " instead of " +
									strings.ToLower(paramType) + "!")
								os.Exit(0)
							}

							primitiveNode.Value = paramValue
							oldParam.Value = paramValue
							if paramType == "BOOLEAN" {
								boolValue := paramValue.(bool)
								primitiveNode.Label = strconv.FormatBool(boolValue)
							} else {
								primitiveNode.Label = paramValue.(string)
							}
							version.Name = nil
							updateNeeded = true
						}
						break
					}
				}
				if !connectionFound {
					fmt.Println(param + " is not connected to any input!")
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
	path = strings.Trim(path, "/")

	if !strings.Contains(path, "/") {
		space := list.GetSpaceByName(spaceName)
		if space == nil {
			os.Exit(0)
		}
		projName := path

		projectExists := false
		for _, proj := range space.Projects {
			if proj.Name == projName {
				projectExists = true
				break
			}
		}
		if !projectExists {
			fmt.Println("Creating a project " + spaceName + "/" + projName +
				" to copy the workflow from the store...")
			create.CreateProject(projName, "", spaceName)
		}
		path = projName + "/" + path
	}
	path = spaceName + "/" + path

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
					if workflowName == "" {
						timeStamp := time.Now().Format(time.RFC3339)
						timeStamp = strings.Replace(timeStamp, "T", "-", 1)
						workflowName = wf.Name + "-" + timeStamp
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
	if maxMachines.Small != nil {
		fmt.Print("Small: ")
		fmt.Println(*maxMachines.Small)
	}
	if maxMachines.Medium != nil {
		fmt.Print("Medium: ")
		fmt.Println(*maxMachines.Medium)
	}
	if maxMachines.Large != nil {
		fmt.Print("Large: ")
		fmt.Println(*maxMachines.Large)
	}
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
