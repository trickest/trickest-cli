package delete

import (
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"trickest-cli/cmd/list"
	"trickest-cli/util"
)

// DeleteCmd represents the delete command
var DeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Deletes an object on the Trickest platform",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Println("You must specify the path of the object to be deleted!")
			return
		}

		space, project, workflow, found := list.ResolveObjectPath(args[0])

		if !found {
			return
		}

		if workflow != nil {
			deleteWorkflow(workflow.ID)
		} else if project != nil {
			deleteProject(project.ID)
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
		var bodyBytes []byte
		bodyBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error: Couldn't read delete space response.")
			return
		}

		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	} else {
		fmt.Println("Space deleted successfully!")
	}
}

func deleteProject(id string) {
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
		var bodyBytes []byte
		bodyBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error: Couldn't read delete project response.")
			return
		}

		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
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
		var bodyBytes []byte
		bodyBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error: Couldn't read delete workflow response.")
			return
		}

		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	} else {
		fmt.Println("Workflow deleted successfully!")
	}
}
