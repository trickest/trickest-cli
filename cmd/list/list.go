package list

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"

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
		space, project, workflow, found := util.GetObjects(args)

		if !found {
			fmt.Println("Error: Not found")
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
			project.Workflows = util.GetWorkflows(project.ID, uuid.Nil, "", false)
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
		output = tree.String()
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

func GetModules(pageSize int, search string) []types.Module {
	urlReq := "library/module/"
	if pageSize > 0 {
		urlReq = urlReq + "?page_size=" + strconv.Itoa(pageSize)
	} else {
		urlReq = urlReq + "?page_size=" + strconv.Itoa(math.MaxInt)
	}

	if search != "" {
		search = url.QueryEscape(search)
		urlReq += "&search=" + search
	}

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get modules!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var modules types.Modules
	err := json.Unmarshal(resp.Body(), &modules)
	if err != nil {
		fmt.Println("Error unmarshalling modules response!")
		return nil
	}

	return modules.Results
}
