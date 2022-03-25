package list

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/xlab/treeprint"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"trickest-cli/types"
	"trickest-cli/util"
)

// ListCmd represents the list command
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists objects on the Trickest platform",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		path := util.FormatPath()
		if len(args) == 0 && path == "" {
			spaces := getSpaces("")

			if spaces != nil && len(spaces) > 0 {
				printSpaces(spaces)
			} else {
				fmt.Println("Couldn't find any spaces!")
			}
			return
		}
		if path == "" {
			path = strings.Trim(args[0], "/")
		} else {
			if len(args) > 0 {
				fmt.Println("Please use either path or flag syntax for the platform objects.")
				return
			}
		}

		space, project, workflow, found := ResolveObjectPath(path)
		if !found {
			return
		}

		if workflow != nil {
			if project != nil && workflow.Name == project.Name {
				if util.WorkflowName == "" {
					printProject(*project)
					if util.ProjectName != "" {
						return
					}
				}
			}
			printWorkflow(*workflow)
		} else if project != nil {
			printProject(*project)
		} else if space != nil {
			printSpaceDetailed(*space)
		}
	},
}

func init() {

}

func printWorkflow(workflow types.Workflow) {
	tree := treeprint.New()
	tree.SetValue("\U0001f9be " + workflow.Name) //🦾
	if workflow.Description != "" {
		tree.AddNode("\U0001f4cb \033[3m" + workflow.Description + "\033[0m") //📋
	}
	tree.AddNode("Author: " + workflow.Author)
	if len(workflow.Parameters) > 0 {
		branch := tree.AddBranch("Parameters")
		for _, param := range workflow.Parameters {
			paramType := strings.ToLower(param.Type)
			if paramType == "boolean" {
				branch.AddNode("[" + paramType + "] " + strconv.FormatBool(param.Value.(bool)))
			} else {
				branch.AddNode("[" + paramType + "] " + param.Value.(string))
			}
		}
	}

	fmt.Println(tree.String())
}

func printProject(project types.Project) {
	tree := treeprint.New()
	tree.SetValue("\U0001f5c2  " + project.Name) //🗂
	if project.Description != "" {
		tree.AddNode("\U0001f4cb \033[3m" + project.Description + "\033[0m") //📋
	}
	if project.Workflows != nil && len(project.Workflows) > 0 {
		wfBranch := tree.AddBranch("Workflows")
		for _, workflow := range project.Workflows {
			wfSubBranch := wfBranch.AddBranch("\U0001f9be " + workflow.Name) //🦾
			if workflow.Description != "" {
				wfSubBranch.AddNode("\U0001f4cb \033[3m" + workflow.Description + "\033[0m") //📋
			}
		}
	}

	fmt.Println(tree.String())
}

func printSpaceDetailed(space types.SpaceDetailed) {
	tree := treeprint.New()
	tree.SetValue("\U0001f4c2 " + space.Name) //📂
	if space.Description != "" {
		tree.AddNode("\U0001f4cb \033[3m" + space.Description + "\033[0m") //📋
	}
	if space.Projects != nil && len(space.Projects) > 0 {
		projBranch := tree.AddBranch("Projects")
		for _, proj := range space.Projects {
			projSubBranch := projBranch.AddBranch("\U0001f5c2  " + proj.Name) //🗂
			if proj.Description != "" {
				projSubBranch.AddNode("\U0001f4cb \033[3m" + proj.Description + "\033[0m") //📋
			}
		}
	}
	if space.Workflows != nil && len(space.Workflows) > 0 {
		wfBranch := tree.AddBranch("Workflows")
		for _, workflow := range space.Workflows {
			wfSubBranch := wfBranch.AddBranch("\U0001f9be " + workflow.Name) //🦾
			if workflow.Description != "" {
				wfSubBranch.AddNode("\U0001f4cb \033[3m" + workflow.Description + "\033[0m") //📋
			}
		}
	}
	fmt.Println(tree.String())
}

func printSpaces(spaces []types.Space) {
	tree := treeprint.New()
	tree.SetValue("Spaces")
	for _, space := range spaces {
		branch := tree.AddBranch("\U0001f4c1 " + space.Name) //📂
		if space.Description != "" {
			branch.AddNode("\U0001f4cb \033[3m" + space.Description + "\033[0m") //📋
		}
	}

	fmt.Println(tree.String())
}

func getSpaces(name string) []types.Space {
	urlReq := util.Cfg.BaseUrl + "v1/spaces/?vault=" + util.GetVault()
	urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)

	if name != "" {
		urlReq += "&name=" + url.QueryEscape(name)
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", urlReq, nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get spaces!")
		os.Exit(0)
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read spaces response body!")
		os.Exit(0)
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var spaces types.Spaces
	err = json.Unmarshal(bodyBytes, &spaces)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal spaces response!")
		os.Exit(0)
	}

	return spaces.Results
}

func GetSpaceByName(name string) *types.SpaceDetailed {
	spaces := getSpaces(name)
	if spaces == nil || len(spaces) == 0 {
		return nil
	}

	return getSpaceByID(spaces[0].ID)
}

func getSpaceByID(id string) *types.SpaceDetailed {
	client := &http.Client{}

	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/spaces/"+id+"/", nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get space by ID!")
		os.Exit(0)
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read space response!")
		os.Exit(0)
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var space types.SpaceDetailed
	err = json.Unmarshal(bodyBytes, &space)
	if err != nil {
		fmt.Println("Error unmarshalling space response!")
		os.Exit(0)
	}

	return &space
}

func GetWorkflows(projectID string, store bool, search string) []types.WorkflowListResponse {
	urlReq := util.Cfg.BaseUrl + "v1/store/workflow/"
	urlReq += "?page_size=" + strconv.Itoa(math.MaxInt)
	if !store {
		urlReq += "&vault=" + util.GetVault()
	}

	if search != "" {
		urlReq += "&search=" + url.QueryEscape(search)
	}

	if projectID != "" {
		urlReq += "&project=" + projectID
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", urlReq, nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get workflows!")
		os.Exit(0)
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read workflows response!")
		os.Exit(0)
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var workflows types.Workflows
	err = json.Unmarshal(bodyBytes, &workflows)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal workflows response!")
		os.Exit(0)
	}

	return workflows.Results
}

func GetWorkflowByID(id string) *types.Workflow {
	client := &http.Client{}

	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/store/workflow/"+id+"/", nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get workflow.")
		os.Exit(0)
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read workflow.")
		os.Exit(0)
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var workflow types.Workflow
	err = json.Unmarshal(bodyBytes, &workflow)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal workflow response!")
		os.Exit(0)
	}

	return &workflow
}

func ResolveObjectPath(path string) (*types.SpaceDetailed, *types.Project, *types.Workflow, bool) {
	pathSplit := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathSplit) > 3 {
		fmt.Println("Invalid object path!")
		return nil, nil, nil, false
	}
	space := GetSpaceByName(pathSplit[0])
	if space == nil {
		fmt.Println("Couldn't find space named " + pathSplit[0] + "!")
		return nil, nil, nil, false
	}

	if len(pathSplit) == 1 {
		return space, nil, nil, true
	}

	var project *types.Project
	if space.Projects != nil && len(space.Projects) > 0 {
		for _, proj := range space.Projects {
			if proj.Name == pathSplit[1] {
				project = &proj
				project.Workflows = GetWorkflows(project.ID, false, "")
				break
			}
		}
	}

	var workflow *types.Workflow
	if space.Workflows != nil && len(space.Workflows) > 0 {
		for _, wf := range space.Workflows {
			if wf.Name == pathSplit[1] {
				workflow = &wf
				break
			}
		}
	}

	if len(pathSplit) == 2 {
		if project != nil || workflow != nil {
			return space, project, workflow, true
		}
		fmt.Println("Couldn't find project or workflow named " + pathSplit[1] + " inside " +
			pathSplit[0] + " space!")
		return space, nil, nil, false
	}

	if project != nil && project.Workflows != nil && len(project.Workflows) > 0 {
		for _, wf := range project.Workflows {
			if wf.Name == pathSplit[2] {
				fullWorkflow := GetWorkflowByID(wf.ID)
				return space, project, fullWorkflow, true
			}
		}
	} else {
		fmt.Println("No workflows found in " + pathSplit[0] + "/" + pathSplit[1])
		return space, project, nil, false
	}

	fmt.Println("Couldn't find workflow named " + pathSplit[2] + " in " + pathSplit[0] + "/" + pathSplit[1] + "/")
	return space, project, nil, false
}

func GetTools(pageSize int, search string, name string) []types.Tool {
	urlReq := util.Cfg.BaseUrl + "v1/store/tool/"
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
		fmt.Println("Error: Couldn't get tools info.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read tools info.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var tools types.Tools
	err = json.Unmarshal(bodyBytes, &tools)
	if err != nil {
		fmt.Println("Error unmarshalling tools response!")
		return nil
	}

	return tools.Results
}
