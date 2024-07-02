package library

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/cmd/list"
	"github.com/trickest/trickest-cli/types"
	"github.com/xlab/treeprint"
)

// libraryListModulesCmd represents the libraryListModules command
var libraryListModulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "List modules from the Trickest library",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		modules := list.GetModules(math.MaxInt, "")
		if len(modules) > 0 {
			printModules(modules, jsonOutput)
		} else {
			fmt.Println("Couldn't find any module in the library!")
			os.Exit(1)
		}
	},
}

func init() {
	libraryListCmd.AddCommand(libraryListModulesCmd)
	libraryListModulesCmd.Flags().BoolVar(&jsonOutput, "json", false, "Display output in JSON format")
}

func printModules(modules []types.Module, jsonOutput bool) {
	var output string

	if jsonOutput {
		data, err := json.Marshal(modules)
		if err != nil {
			fmt.Println("Error marshalling response data")
			return
		}
		output = string(data)
	} else {
		tree := treeprint.New()
		tree.SetValue("Modules")
		for _, module := range modules {
			mdSubBranch := tree.AddBranch("\U0001f916 " + module.Name) //🤖
			if module.Description != "" {
				mdSubBranch.AddNode("\U0001f4cb \033[3m" + module.Description + "\033[0m") //📋
			}
			inputSubBranch := mdSubBranch.AddBranch("\U0001f4e5 Inputs") //📥
			for _, input := range module.Data.Inputs {
				inputSubBranch.AddNode(input.Name)
			}
			outputSubBranch := mdSubBranch.AddBranch("\U0001f4e4 Outputs") //📤
			for _, output := range module.Data.Outputs {
				outputSubBranch.AddNode(output.Name)
			}
		}

		output = tree.String()
	}
	fmt.Println(output)
}
