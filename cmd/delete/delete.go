package delete

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/cmd/list"
	"github.com/trickest/trickest-cli/util"

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

		space, project, workflow, found := list.ResolveObjectPath(path, false, util.WorkflowName == "")
		if !found {
			return
		}

		if workflow != nil {
			deleteWorkflow(workflow.ID)
		} else if project != nil {
			DeleteProject(project.ID)
		} else if space != nil {
			deleteSpace("", space.ID)
		}
	},
}

func deleteSpace(name string, id uuid.UUID) {
	if id == uuid.Nil {
		space := list.GetSpaceByName(name)
		if space == nil {
			fmt.Println("Couldn't find space named " + name + "!")
			os.Exit(0)
		}
		id = space.ID
	}

	resp := request.Trickest.Delete().DoF("spaces/%s/", id.String())
	if resp == nil {
		fmt.Println("Couldn't delete space with ID: " + id.String())
		os.Exit(0)
	}

	if resp.Status() != http.StatusNoContent {
		request.ProcessUnexpectedResponse(resp)
	} else {
		fmt.Println("Space deleted successfully!")
	}
}

func DeleteProject(id uuid.UUID) {
	resp := request.Trickest.Delete().DoF("projects/%s/", id.String())
	if resp == nil {
		fmt.Println("Couldn't delete project with ID: " + id.String())
		os.Exit(0)
	}

	if resp.Status() != http.StatusNoContent {
		request.ProcessUnexpectedResponse(resp)
	} else {
		fmt.Println("Project deleted successfully!")
	}
}

func deleteWorkflow(id uuid.UUID) {
	resp := request.Trickest.Delete().DoF("store/workflow/%s/", id.String())
	if resp == nil {
		fmt.Println("Couldn't delete workflow with ID: " + id.String())
		os.Exit(0)
	}

	if resp.Status() != http.StatusNoContent {
		request.ProcessUnexpectedResponse(resp)
	} else {
		fmt.Println("Workflow deleted successfully!")
	}
}
