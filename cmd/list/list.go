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
	"trickest-cli/types"
	"trickest-cli/util"
)

// listCmd represents the list command
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists objects on the Trickest platform",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			spaces := getSpaces("")

			if spaces != nil && len(spaces) > 0 {
				printSpaces(spaces)
			} else {
				fmt.Println("Couldn't find any spaces!")
			}
			return
		}

		space := getSpaceByName(args[0])
		if space != nil {
			printSpaceDetailed(*space)
		} else {
			fmt.Println("Couldn't find space with the given name!")
		}
	},
}

func init() {

}

func printSpaceDetailed(space types.SpaceDetailed) {
	tree := treeprint.New()
	tree.SetValue("\U0001f4c2 " + space.Name) //📂
	if space.Description != "" {
		tree.AddNode("\U0001f4cb " + space.Description) //📋
	}
	if space.Projects != nil && len(space.Projects) > 0 {
		projBranch := tree.AddBranch("Projects")
		for _, proj := range space.Projects {
			projSubBranch := projBranch.AddBranch("\U0001f5c2  " + proj.Name) //🗂
			if proj.Description != "" {
				projSubBranch.AddNode("\U0001f4cb " + proj.Description) //📋
			}
		}
	}
	if space.Workflows != nil && len(space.Workflows) > 0 {
		wfBranch := tree.AddBranch("Workflows")
		for _, workflow := range space.Workflows {
			wfSubBranch := wfBranch.AddBranch("\U0001f6e0  " + workflow.Name) //🛠️
			if workflow.Description != "" {
				wfSubBranch.AddNode("\U0001f4cb " + workflow.Description) //📋
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
			branch.AddNode("\U0001f4cb " + space.Description) //📋
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

func getSpaceByName(name string) *types.SpaceDetailed {
	spaces := getSpaces(name)
	if spaces == nil || len(spaces) == 0 {
		fmt.Println("Couldn't find space with the given name!")
		os.Exit(0)
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
