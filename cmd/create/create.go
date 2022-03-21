package create

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"trickest-cli/cmd/list"
	"trickest-cli/types"
	"trickest-cli/util"
)

var description string

// CreateCmd represents the create command
var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a space or a project on the Trickest platform",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		path := util.FormatPath()
		if path == "" {
			if len(args) == 0 {
				fmt.Println("You must specify the object to be created!")
				return
			}
			path = strings.Trim(args[0], "/")
		}

		pathSplit := strings.Split(path, "/")
		if len(pathSplit) > 2 {
			fmt.Println("Only space or space/project should be specified!")
			return
		}

		if len(pathSplit) == 1 {
			createSpace(pathSplit[0], description)
		} else {
			CreateProject(pathSplit[1], description, pathSplit[0])
		}

	},
}

func init() {
	CreateCmd.PersistentFlags().StringVarP(&description, "description", "d", "", "Space description")
}

func createSpace(name string, description string) {
	space := types.CreateSpaceRequest{
		Name:        name,
		Description: description,
		VaultInfo:   util.GetVault(),
	}

	buf := new(bytes.Buffer)

	err := json.NewEncoder(buf).Encode(&space)
	if err != nil {
		fmt.Println("Error encoding create space request!")
		return
	}

	bodyData := bytes.NewReader(buf.Bytes())

	client := &http.Client{}
	req, err := http.NewRequest("POST", util.Cfg.BaseUrl+"v1/spaces/", bodyData)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't create space.")
		return
	}

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read response body!")
		return
	}

	if resp.StatusCode != http.StatusCreated {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	fmt.Println("Space successfully created! ")
}

func CreateProject(name string, description string, spaceName string) *types.Project {
	space := list.GetSpaceByName(spaceName)
	if space == nil {
		fmt.Println("The space \"" + spaceName + "\" doesn't exist. Would you like to create it? (Y/N)")
		var answer string
		for {
			_, _ = fmt.Scan(&answer)
			if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
				createSpace(spaceName, "")
				return CreateProject(name, description, spaceName)
			} else if strings.ToLower(answer) == "n" || strings.ToLower(answer) == "no" {
				os.Exit(0)
			}
		}
	}

	project := types.CreateProjectRequest{
		Name:        name,
		Description: description,
		SpaceID:     space.ID,
	}

	buf := new(bytes.Buffer)

	err := json.NewEncoder(buf).Encode(&project)
	if err != nil {
		fmt.Println("Error encoding create project request!")
		os.Exit(0)
	}

	bodyData := bytes.NewReader(buf.Bytes())

	client := &http.Client{}
	req, err := http.NewRequest("POST", util.Cfg.BaseUrl+"v1/projects/", bodyData)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't create project.")
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

	fmt.Println("Project successfully created!")
	var newProject types.Project
	err = json.Unmarshal(bodyBytes, &newProject)
	if err != nil {
		fmt.Println("Error unmarshalling create project response!")
		os.Exit(0)
	}

	return &newProject
}

func CreateProjectIfNotExists(space *types.SpaceDetailed, projectName string) *types.Project {
	for _, proj := range space.Projects {
		if proj.Name == projectName {
			return &proj
		}
	}
	return CreateProject(projectName, "", space.Name)
}

func CreateWorkflow(name, description, spaceID, projectID string) *types.CreateWorkflowResponse {
	workflow := types.CreateWorkflowRequest{
		Name:        name,
		Description: description,
		SpaceID:     spaceID,
		ProjectID:   projectID,
	}

	buf := new(bytes.Buffer)

	err := json.NewEncoder(buf).Encode(&workflow)
	if err != nil {
		fmt.Println("Error encoding create workflow request!")
		os.Exit(0)
	}

	bodyData := bytes.NewReader(buf.Bytes())

	client := &http.Client{}
	req, err := http.NewRequest("POST", util.Cfg.BaseUrl+"v1/store/workflow/", bodyData)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't create workflow.")
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

	fmt.Println("Workflow successfully created!")
	var createWorkflowResp types.CreateWorkflowResponse
	err = json.Unmarshal(bodyBytes, &createWorkflowResp)
	if err != nil {
		fmt.Println("Error unmarshalling create workflow response!")
		os.Exit(0)
	}

	return &createWorkflowResp
}
