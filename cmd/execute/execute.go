package execute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"net/http"
	"os"
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

		workflow := prepareForExec(args[0])
		getAvailableMachines()
		getMaxMachines(workflow)
		readConfig(configFile)

	},
}

func init() {
	ExecuteCmd.Flags().StringVar(&spaceName, "space", "Playground", "Space name")
	ExecuteCmd.Flags().StringVar(&workflowName, "name", "", "Workflow name")
	ExecuteCmd.Flags().StringVar(&configFile, "config", "", "YAML file for run configuration")
	ExecuteCmd.Flags().BoolVar(&watch, "watch", false, "Watch the execution running")

}

func readConfig(fileName string) {
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Couldn't open config file!")
		return
	}

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("Couldn't read config!")
		return
	}

	var config map[string]interface{}
	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		fmt.Println("Couldn't unmarshal config!")
		return
	}

	if machines, exists := config["machines"]; exists {
		machinesList := machines.([]interface{})

		for _, m := range machinesList {
			machine := m.(map[string]interface{})

			if small, ok := machine["small"]; ok {
				if n, number := small.(int); number {
					if n != 0 {
						executionMachines.Small = &n
					}
				} else if s, word := small.(string); word {
					if strings.ToLower(s) == "max" || strings.ToLower(s) == "maximum" {
						executionMachines.Small = maxMachines.Small
					} else {
						processInvalidMachineString(s)
					}
				} else {
					processInvalidMachineType(small)
				}
			}

			if medium, ok := machine["medium"]; ok {
				if n, number := medium.(int); number {
					if n != 0 {
						executionMachines.Medium = &n
					}
				} else if s, word := medium.(string); word {
					if strings.ToLower(s) == "max" || strings.ToLower(s) == "maximum" {
						executionMachines.Medium = maxMachines.Medium
					} else {
						processInvalidMachineString(s)
					}
				} else {
					processInvalidMachineType(medium)
				}
			}

			if large, ok := machine["large"]; ok {
				if n, number := large.(int); number {
					if n != 0 {
						executionMachines.Large = &n
					}
				} else if s, word := large.(string); word {
					if strings.ToLower(s) == "max" || strings.ToLower(s) == "maximum" {
						executionMachines.Large = maxMachines.Large
					} else {
						processInvalidMachineString(s)
					}
				} else {
					processInvalidMachineType(large)
				}
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
	} else {
		executionMachines = maxMachines
	}

}

func getMaxMachines(workflow *types.Workflow) {
	if workflow == nil {
		os.Exit(0)
	}

	version := getLatestWorkflowVersion(workflow)

	if version == nil {
		os.Exit(0)
	}

	maxMachines = version.MaxMachines
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
			fmt.Println("Use \"trickest store list\" to see all available workflows,\n" +
				"or search the store using \"trickest store search <name/description>\"")
			os.Exit(0)
		}
	}

	fmt.Println("Workflow path: " + workflow.SpaceName + "/" + workflow.ProjectName + "/" + workflow.Name)

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

	if resp.StatusCode != 201 {
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

	if resp.StatusCode != 200 {
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
}

func processInvalidMachineType(data interface{}) {
	fmt.Print("Invalid machine qualifier:")
	fmt.Println(data)
	fmt.Println("Try using a number or max/maximum instead.")
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
