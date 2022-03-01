package execute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
	"trickest-cli/cmd/create"
	"trickest-cli/cmd/list"
	"trickest-cli/types"
	"trickest-cli/util"
)

var (
	spaceName    string
	workflowName string
	configFile   string
	watch        bool
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

		path := strings.Trim(args[0], "/")

		if !strings.Contains(path, "/") {
			space := list.GetSpaceByName(spaceName)
			if space == nil {
				return
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
			return
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
							return
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
				return
			}
		}

		fmt.Println("Workflow path: " + workflow.SpaceName + "/" + workflow.ProjectName + "/" + workflow.Name)
	},
}

func init() {
	ExecuteCmd.Flags().StringVar(&spaceName, "space", "Playground", "Space name")
	ExecuteCmd.Flags().StringVar(&workflowName, "name", "", "Workflow name")
	ExecuteCmd.Flags().StringVar(&configFile, "config", "", "YAML file for run configuration")
	ExecuteCmd.Flags().BoolVar(&watch, "watch", false, "Watch the execution running")

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
