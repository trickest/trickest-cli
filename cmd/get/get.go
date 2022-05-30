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

var (
	watch          bool
	showNodeParams bool
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
		} else {
			if len(args) > 0 {
				fmt.Println("Please use either path or flag syntax for the platform objects.")
				return
			}
		}

		_, _, workflow, found := list.ResolveObjectPath(path, false)
		if !found {
			return
		}

		version := execute.GetLatestWorkflowVersion(workflow)
		allNodes, roots := execute.CreateTrees(version, false)

		runs := download.GetRuns(version.WorkflowInfo, 1)
		if runs != nil && len(runs) > 0 && runs[0].Status == "RUNNING" {
			execute.WatchRun(runs[0].ID, map[string]download.NodeInfo{}, !watch, &runs[0].Bees, showNodeParams)
			return
		} else {
			const fmtStr = "%-15s %v\n"
			out := ""
			out += fmt.Sprintf(fmtStr, "Name:", workflow.Name)
			status := "no runs"
			if runs != nil && len(runs) > 0 {
				status = strings.ToLower(runs[0].Status)
			}
			out += fmt.Sprintf(fmtStr, "Status:", status)
			availableBees := execute.GetAvailableMachines()
			out += fmt.Sprintf(fmtStr, "Max machines:", execute.FormatMachines(&version.MaxMachines, true)+
				" (currently available: "+execute.FormatMachines(&availableBees, true)+")")
			out += fmt.Sprintf(fmtStr, "Created:", workflow.CreatedDate.In(time.Local).Format(time.RFC1123)+
				" ("+util.FormatDuration(time.Since(workflow.CreatedDate))+" ago)")

			for _, node := range allNodes {
				node.Status = ""
			}
			out += "\n" + execute.PrintTrees(roots, &allNodes, showNodeParams, false)
			fmt.Print(out)
		}

	},
}

func init() {
	GetCmd.Flags().BoolVar(&watch, "watch", false, "Watch the workflow execution if it's still running")
	GetCmd.Flags().BoolVar(&showNodeParams, "show-params", false, "Show parameters in the workflow tree")
}
