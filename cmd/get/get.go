package get

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/cmd/execute"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var (
	watch          bool
	showNodeParams bool
	runID          string
	jsonOutput     bool
)

// GetCmd represents the get command
var GetCmd = &cobra.Command{
	Use:   "get",
	Short: "Displays status of a workflow",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		_, _, workflow, found := util.GetObjects(args)

		if !found || workflow == nil {
			fmt.Println("Error: Workflow not found")
			return
		}

		version := execute.GetLatestWorkflowVersion(workflow.ID, uuid.Nil)
		allNodes, roots := execute.CreateTrees(version, false)

		var runs []types.Run
		if runID == "" && util.URL != "" {
			workflowURLRunID, err := util.GetRunIDFromWorkflowURL(util.URL)
			if err == nil {
				runID = workflowURLRunID
			}
		}
		if runID == "" {
			runs = util.GetRuns(version.WorkflowInfo, 1, "")
		} else {
			runUUID, err := uuid.Parse(runID)
			if err != nil {
				fmt.Println("Invalid run ID")
				return
			}
			run := execute.GetRunByID(runUUID)
			runs = []types.Run{*run}
		}
		if len(runs) > 0 {
			run := runs[0]
			if run.Status == "COMPLETED" && run.CompletedDate.IsZero() {
				run.Status = "RUNNING"
			}

			ipAddresses, err := getRunIPAddresses(*run.ID)
			if err != nil {
				fmt.Printf("Warning: Couldn't get the run IP addresses: %s", err)
			}
			run.IPAddresses = ipAddresses

			if jsonOutput {
				data, err := json.Marshal(run)
				if err != nil {
					fmt.Println("Error marshalling project data")
					return
				}
				output := string(data)
				fmt.Println(output)
			} else {
				execute.WatchRun(*run.ID, "", []string{}, []string{}, !watch, &runs[0].Machines, showNodeParams)
			}
			return
		} else {
			if jsonOutput {
				// No runs
				fmt.Println("{}")
			} else {
				const fmtStr = "%-15s %v\n"
				out := ""
				out += fmt.Sprintf(fmtStr, "Name:", workflow.Name)
				status := "no runs"
				if len(runs) > 0 {
					status = strings.ToLower(runs[0].Status)
				}
				out += fmt.Sprintf(fmtStr, "Status:", status)
				out += fmt.Sprintf(fmtStr, "Created:", workflow.CreatedDate.In(time.Local).Format(time.RFC1123)+
					" ("+util.FormatDuration(time.Since(workflow.CreatedDate))+" ago)")

				for _, node := range allNodes {
					node.Status = ""
				}
				out += "\n" + execute.PrintTrees(roots, &allNodes, showNodeParams, false)
				fmt.Print(out)
			}
		}

	},
}

func init() {
	GetCmd.Flags().BoolVar(&watch, "watch", false, "Watch the workflow execution if it's still running")
	GetCmd.Flags().BoolVar(&showNodeParams, "show-params", false, "Show parameters in the workflow tree")
	GetCmd.Flags().StringVar(&runID, "run", "", "Get the status of a specific run")
	GetCmd.Flags().BoolVar(&jsonOutput, "json", false, "Display output in JSON format")
}

func getRunIPAddresses(runID uuid.UUID) ([]string, error) {
	resp := request.Trickest.Get().DoF("execution/%s/ips/", runID)
	if resp == nil || resp.Status() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status code: %d", resp.Status())
	}

	var ipAddresses []string
	err := json.Unmarshal(resp.Body(), &ipAddresses)
	if err != nil {
		return nil, fmt.Errorf("couldn't unmarshal IP addresses response: %s", err)
	}

	return ipAddresses, nil
}
