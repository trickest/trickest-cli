package list

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
	"github.com/xlab/treeprint"
)

var (
	jsonOutput bool
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
				printSpaces(spaces, jsonOutput)
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

		var (
			space    *types.SpaceDetailed
			project  *types.Project
			workflow *types.Workflow
			found    bool
		)
		if util.WorkflowName == "" {
			space, project, workflow, found = ResolveObjectPath(path, false, true)
		} else {
			space, project, workflow, found = ResolveObjectPath(path, false, false)
		}
		if !found {
			return
		}

		if workflow != nil {
			if project != nil && workflow.Name == project.Name {
				if util.WorkflowName == "" {
					printProject(*project, jsonOutput)
					if util.ProjectName != "" {
						return
					}
				}
			}
			printWorkflow(*workflow, jsonOutput)
		} else if project != nil {
			printProject(*project, jsonOutput)
		} else if space != nil {
			printSpaceDetailed(*space)
		}
	},
}

func init() {
	ListCmd.Flags().BoolVar(&jsonOutput, "json", false, "Display output in JSON format")
}

func printWorkflow(workflow types.Workflow, jsonOutput bool) {
	var output string

	if jsonOutput {
		data, err := json.Marshal(workflow)
		if err != nil {
			fmt.Println("Error marshalling workflow data")
			return
		}
		output = string(data)
	} else {
		tree := treeprint.New()
		tree.SetValue("\U0001f9be " + workflow.Name) //🦾
		if workflow.Description != "" {
			tree.AddNode("\U0001f4cb \033[3m" + workflow.Description + "\033[0m") //📋
		}
		tree.AddNode("Author: " + workflow.Author)
		output = tree.String()
	}

	fmt.Println(output)
}

func printProject(project types.Project, jsonOutput bool) {
	var output string

	if jsonOutput {
		data, err := json.Marshal(project)
		if err != nil {
			fmt.Println("Error marshalling project data")
			return
		}
		output = string(data)
	} else {
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

	}

	fmt.Println(output)
}

func printSpaceDetailed(space types.SpaceDetailed) {
	var output string

	if jsonOutput {
		data, err := json.Marshal(space)
		if err != nil {
			fmt.Println("Error marshalling space data")
			return
		}
		output = string(data)
	} else {
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
		output = tree.String()
	}
	fmt.Println(output)
}

func printSpaces(spaces []types.Space, jsonOutput bool) {
	var output string

	if jsonOutput {
		data, err := json.Marshal(spaces)
		if err != nil {
			fmt.Println("Error marshalling spaces data")
			return
		}
		output = string(data)
	} else {
		tree := treeprint.New()
		tree.SetValue("Spaces")
		for _, space := range spaces {
			branch := tree.AddBranch("\U0001f4c1 " + space.Name) //📂
			if space.Description != "" {
				branch.AddNode("\U0001f4cb \033[3m" + space.Description + "\033[0m") //📋
			}
		}

		output = tree.String()
	}

	fmt.Println(output)
}

func getSpaces(name string) []types.Space {
	urlReq := "spaces/?vault=" + util.GetVault().String()
	urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)

	if name != "" {
		urlReq += "&name=" + url.QueryEscape(name)
	}

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get spaces!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var spaces types.Spaces
	err := json.Unmarshal(resp.Body(), &spaces)
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

func getSpaceByID(id uuid.UUID) *types.SpaceDetailed {
	resp := request.Trickest.Get().DoF("spaces/%s/", id.String())
	if resp == nil {
		fmt.Println("Error: Couldn't get space by ID!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var space types.SpaceDetailed
	err := json.Unmarshal(resp.Body(), &space)
	if err != nil {
		fmt.Println("Error unmarshalling space response!")
		os.Exit(0)
	}

	return &space
}

func GetWorkflows(projectID, spaceID uuid.UUID, search string, library bool) []types.Workflow {
	urlReq := "workflow/"
	if library {
		urlReq = "library/" + urlReq
	}
	urlReq += "?page_size=" + strconv.Itoa(math.MaxInt)
	if !library {
		urlReq += "&vault=" + util.GetVault().String()
	}

	if search != "" {
		urlReq += "&search=" + url.QueryEscape(search)
	}

	if projectID != uuid.Nil {
		urlReq += "&project=" + projectID.String()
	} else if spaceID != uuid.Nil {
		urlReq += "&space=" + spaceID.String()
	}

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get workflows!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var workflows types.Workflows
	err := json.Unmarshal(resp.Body(), &workflows)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal workflows response!")
		os.Exit(0)
	}

	return workflows.Results
}

func GetWorkflowByID(id uuid.UUID) *types.Workflow {
	resp := request.Trickest.Get().DoF("workflow/%s/", id.String())
	if resp == nil {
		fmt.Println("Error: Couldn't get workflow by ID!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var workflow types.Workflow
	err := json.Unmarshal(resp.Body(), &workflow)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal workflow response!")
		os.Exit(0)
	}

	return &workflow
}

func ResolveObjectPath(path string, silent bool, isProject bool) (*types.SpaceDetailed, *types.Project, *types.Workflow, bool) {
	pathSplit := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathSplit) > 3 {
		if !silent {
			fmt.Println("Invalid object path!")
		}
		return nil, nil, nil, false
	}
	space := GetSpaceByName(pathSplit[0])
	if space == nil {
		if !silent {
			fmt.Println("Couldn't find space named " + pathSplit[0] + "!")
		}
		return nil, nil, nil, false
	}

	if len(pathSplit) == 1 {
		return space, nil, nil, true
	}

	// Space and workflow with no project
	var projectName string
	var workflowName string
	if len(pathSplit) == 2 {
		if isProject {
			projectName = pathSplit[1]
			workflowName = ""
		} else {
			projectName = ""
			workflowName = pathSplit[1]
		}
	} else {
		projectName = pathSplit[1]
		workflowName = pathSplit[2]
	}

	var project *types.Project
	if space.Projects != nil && len(space.Projects) > 0 {
		for _, proj := range space.Projects {
			if proj.Name == projectName {
				project = &proj
				project.Workflows = GetWorkflows(project.ID, uuid.Nil, "", false)
				break
			}
		}
	}

	var workflow *types.Workflow
	if space.Workflows != nil && len(space.Workflows) > 0 {
		for _, wf := range space.Workflows {
			if wf.Name == workflowName {
				workflow = &wf
				break
			}
		}
	}

	if len(pathSplit) == 2 {
		if project != nil || workflow != nil {
			return space, project, workflow, true
		}
		if workflow != nil {
			return space, nil, workflow, true
		}
		if !silent {
			fmt.Println("Couldn't find project or workflow named " + pathSplit[1] + " inside " +
				pathSplit[0] + " space!")
		}
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
		if !silent {
			fmt.Println("No workflows found in " + pathSplit[0] + "/" + pathSplit[1])
		}
		return space, project, nil, false
	}

	if !silent {
		fmt.Println("Couldn't find workflow named " + pathSplit[2] + " in " + pathSplit[0] + "/" + pathSplit[1] + "/")
	}
	return space, project, nil, false
}

func GetTools(pageSize int, search string, name string) []types.Tool {
	urlReq := "library/tool/"
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

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get tools!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var tools types.Tools
	err := json.Unmarshal(resp.Body(), &tools)
	if err != nil {
		fmt.Println("Error unmarshalling tools response!")
		return nil
	}

	return tools.Results
}
