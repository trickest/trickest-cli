package get

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"
	"time"
	"trickest-cli/cmd/download"
	"trickest-cli/cmd/execute"
	"trickest-cli/cmd/list"
	"trickest-cli/util"
)

// GetCmd represents the get command
var GetCmd = &cobra.Command{
	Use:   "get",
	Short: "Displays status of a workflow",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		path := util.FormatPath()
		if path == "" {
			if len(args) == 0 {
				fmt.Println("Workflow path must be specified!")
				return
			}
			path = strings.Trim(args[0], "/")
		}

		_, _, workflow, found := list.ResolveObjectPath(path)
		if !found {
			return
		}

		version := execute.GetLatestWorkflowVersion(workflow)
		allNodes, roots := execute.CreateTrees(version)

		if version.RunCount > 0 {
			runs := download.GetRuns(version.WorkflowInfo, 1)
			if runs == nil || len(runs) == 0 {
				fmt.Println("Couldn't get latest run!")
				return
			}
			execute.WatchRun(runs[0].ID, map[string]download.NodeInfo{}, true, &runs[0].Bees)
			return
		} else {
			const fmtStr = "%-15s %v\n"
			out := ""
			out += fmt.Sprintf(fmtStr, "Name:", workflow.Name)
			out += fmt.Sprintf(fmtStr, "Status:", "no runs")
			availableBees := execute.GetAvailableMachines()
			out += fmt.Sprintf(fmtStr, "Max machines:", execute.FormatMachines(&version.MaxMachines, true)+
				" (currently available: "+execute.FormatMachines(&availableBees, true)+")")
			out += fmt.Sprintf(fmtStr, "Created:", workflow.CreatedDate.In(time.Local).Format(time.RFC1123)+
				" ("+time.Since(workflow.CreatedDate).Round(time.Second).String()+" ago)")

			for _, node := range allNodes {
				node.Status = ""
			}
			out += "\n" + execute.PrintTrees(roots, &allNodes, false, false)
			fmt.Print(out)
		}

	},
}

func init() {

}