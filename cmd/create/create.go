package create

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/cmd/delete"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
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
		} else {
			if len(args) > 0 {
				fmt.Println("Please use either path or flag syntax for the platform objects.")
				return
			}
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

	data, err := json.Marshal(space)
	if err != nil {
		fmt.Println("Error marshaling create space request!")
		return
	}

	resp := request.Trickest.Post().Body(data).DoF("spaces/?vault=%s", util.GetVault())
	if resp == nil {
		fmt.Println("Error: Couldn't create space.")
		return
	}

	if resp.Status() != http.StatusCreated {
		request.ProcessUnexpectedResponse(resp)
	}

	fmt.Println("Space successfully created! ")
}

func CreateProject(name string, description string, spaceName string) *types.Project {
	space := util.GetSpaceByName(spaceName)
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

	data, err := json.Marshal(project)
	if err != nil {
		fmt.Println("Error marshaling create project request!")
		os.Exit(0)
	}

	resp := request.Trickest.Post().Body(data).DoF("projects/?vault=%s", util.GetVault())
	if resp == nil {
		fmt.Println("Error: Couldn't create project.")
		os.Exit(0)
	}

	if resp.Status() != http.StatusCreated {
		request.ProcessUnexpectedResponse(resp)
	}

	fmt.Println("Project successfully created!")
	var newProject types.Project
	err = json.Unmarshal(resp.Body(), &newProject)
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

func CreateWorkflow(name, description string, spaceID, projectID uuid.UUID, deleteProjectOnError bool) *types.CreateWorkflowResponse {
	workflow := types.CreateWorkflowRequest{
		Name:        name,
		Description: description,
		SpaceID:     spaceID,
	}

	if projectID != uuid.Nil {
		workflow.ProjectID = &projectID
	}

	workflows := util.GetWorkflows(projectID, spaceID, name, false)
	if workflows != nil {
		for _, wf := range workflows {
			if wf.Name == name {
				fmt.Println(name + ": A workflow with the same name already exists.")
				os.Exit(0)
			}
		}
	}

	data, err := json.Marshal(workflow)
	if err != nil {
		fmt.Println("Error marshaling create workflow request!")
		os.Exit(0)
	}

	resp := request.Trickest.Post().Body(data).DoF("workflow/?vault=%s", util.GetVault())
	if resp == nil {
		fmt.Println("Error: Couldn't create workflow.")
		os.Exit(0)
	}

	if resp.Status() != http.StatusCreated {
		if deleteProjectOnError && projectID != uuid.Nil {
			delete.DeleteProject(projectID)
		}
		request.ProcessUnexpectedResponse(resp)
	}

	fmt.Print("Workflow successfully created!\n\n")
	var createWorkflowResp types.CreateWorkflowResponse
	err = json.Unmarshal(resp.Body(), &createWorkflowResp)
	if err != nil {
		fmt.Println("Error unmarshalling create workflow response!")
		os.Exit(0)
	}

	return &createWorkflowResp
}
