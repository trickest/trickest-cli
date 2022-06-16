package delete

import (
	"fmt"
	"net/http"
	"strings"
	"trickest-cli/cmd/list"
	"trickest-cli/types"
	"trickest-cli/util"

	"github.com/spf13/cobra"
)

// DeleteCmd represents the delete command
var DeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Deletes an object on the Trickest platform",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		path := util.FormatPath()
		if path == "" {
			if len(args) == 0 {
				fmt.Println("You must specify the path of the object to be deleted!")
				return
			}
			path = strings.Trim(args[0], "/")
		} else {
			if len(args) > 0 {
				fmt.Println("Please use either path or flag syntax for the platform objects.")
				return
			}
		}

		var (
			space    *types.SpaceDetailed
			project  *types.Project
			workflow *types.Workflow
			found    bool
		)
		if util.WorkflowName == "" {
			space, project, workflow, found = list.ResolveObjectPath(path, false, true)
		} else {
			space, project, workflow, found = list.ResolveObjectPath(path, false, false)
		}
		if !found {
			return
		}

		if workflow != nil {
			deleteWorkflow(workflow.ID)
		} else if project != nil {
			DeleteProject(project.ID)
			return
		} else if space != nil {
			deleteSpace("", space.ID)
		}
	},
}

func init() {

}

func deleteSpace(name string, id string) {
	if id == "" {
		space := list.GetSpaceByName(name)
		if space == nil {
			fmt.Println("Couldn't find space named " + name + "!")
			return
		}
		id = space.ID
	}

	client := &http.Client{}

	req, err := http.NewRequest("DELETE", util.Cfg.BaseUrl+"v1/spaces/"+id+"/", nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't delete space with ID: " + id)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		util.ProcessUnexpectedResponse(resp)
	} else {
		fmt.Println("Space deleted successfully!")
	}
}

func DeleteProject(id string) {
	client := &http.Client{}

	req, err := http.NewRequest("DELETE", util.Cfg.BaseUrl+"v1/projects/"+id+"/", nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't delete project with ID: " + id)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		util.ProcessUnexpectedResponse(resp)
	} else {
		fmt.Println("Project deleted successfully!")
	}
}

func deleteWorkflow(id string) {
	client := &http.Client{}

	req, err := http.NewRequest("DELETE", util.Cfg.BaseUrl+"v1/store/workflow/"+id+"/", nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't delete workflow with ID: " + id)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		util.ProcessUnexpectedResponse(resp)
	} else {
		fmt.Println("Workflow deleted successfully!")
	}
}
