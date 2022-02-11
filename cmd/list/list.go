package list

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/xlab/treeprint"
	"io/ioutil"
	"math"
	"net/http"
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
		spaces := getSpaces()

		if spaces != nil && len(spaces) > 0 {
			printSpaces(spaces)
		} else {
			fmt.Println("Couldn't find any spaces!")
		}
	},
}

func init() {

}

func printSpaces(spaces []types.Space) {
	tree := treeprint.New()
	tree.SetValue("Spaces")
	for _, space := range spaces {
		branch := tree.AddBranch("\U0001f4c1 " + space.Name) //📂
		if space.Description != "" {
			branch.AddNode("\U0001f5c2 " + space.Description) //🗂
		}
	}

	fmt.Println(tree.String())
}

func getSpaces() []types.Space {
	urlReq := util.Cfg.BaseUrl + "v1/spaces/?vault=" + util.GetVault()
	urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)

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
		os.Exit(0)
	}

	var spaces types.Spaces
	err = json.Unmarshal(bodyBytes, &spaces)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal spaces response!")
		os.Exit(0)
	}

	return spaces.Results
}
