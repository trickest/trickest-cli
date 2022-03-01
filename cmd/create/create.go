package create

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
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
		if len(args) != 1 {
			fmt.Println("You must specify the path of the object to be created!")
			return
		}

		pathSplit := strings.Split(strings.Trim(args[0], "/"), "/")
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

func CreateProject(name string, description string, spaceName string) {
	space := list.GetSpaceByName(spaceName)
	if space == nil {
		fmt.Println("The space \"" + spaceName + "\" doesn't exist. Would you like to create it? (Y/N)")
		var answer string
		for {
			_, _ = fmt.Scan(&answer)
			if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
				createSpace(spaceName, "")
				CreateProject(name, description, spaceName)
				return
			} else if strings.ToLower(answer) == "n" || strings.ToLower(answer) == "no" {
				return
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
		return
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

	fmt.Println("Project successfully created!")
}
