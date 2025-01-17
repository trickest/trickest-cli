package stop

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"
)

var (
	allRuns      bool
	runID        string
	nodesFlag    string
	childrenFlag string
)

// StopCmd represents the export command
var StopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a workflow/node",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		_, _, workflow, found := util.GetObjects(args)
		if !found {
			return
		}

		var nodes []string
		if nodesFlag != "" {
			for _, node := range strings.Split(nodesFlag, ",") {
				nodes = append(nodes, strings.ReplaceAll(node, "/", "-"))
			}
		} else if util.URL != "" {
			node, err := util.GetNodeIDFromWorkflowURL(util.URL)
			if err == nil {
				nodes = append(nodes, node)
			}
		}

		var children []string
		if childrenFlag != "" {
			children = strings.Split(childrenFlag, ",")
		}

		runs, err := getRelevantRuns(*workflow, allRuns, runID, util.URL)
		if err != nil {
			fmt.Printf("Couldn't get the specified in progress workflow run(s): %v\n", err)
			return
		}
		if len(runs) == 0 {
			fmt.Printf("No runs are currently in progress for the workflow: %s (ID: %s)\n", workflow.Name, workflow.ID)
			return
		}

		var errs []error
		for _, run := range runs {
			if len(nodes) == 0 && len(children) == 0 {
				err = stopRun(run)
				if err != nil {
					errs = append(errs, err)
				} else {
					fmt.Printf("Successfully sent stop request for run %s\n", run.ID)
				}
			} else {
				version := util.GetWorkflowVersionByID(*run.WorkflowVersionInfo, uuid.Nil)
				subJobs := util.GetSubJobs(*run.ID)
				subJobs = util.LabelSubJobs(subJobs, *version)

				if len(nodes) > 0 && len(children) == 0 {
					for _, subJob := range subJobs {
						labelExists := slices.Contains(nodes, subJob.Label)
						nameExists := slices.Contains(nodes, subJob.Name)
						if nameExists || labelExists {
							err = stopSubJob(subJob)
							if err != nil {
								errs = append(errs, err)
							} else {
								fmt.Printf("Successfully sent stop request for node \"%s\" in run %s\n", subJob.Label, run.ID)
							}
						}
					}
				} else {
					for _, node := range nodes {
						for _, child := range children {
							err = stopSubJobChild(run, node, child)
							if err != nil {
								errs = append(errs, err)
							} else {
								fmt.Printf("Successfully sent stop request for child %s in node \"%s\" in run %s\n", child, node, run.ID)
							}
						}
					}
				}
			}
		}

		if len(errs) > 0 {
			fmt.Println("errors encountered while stopping runs:")
			for _, err := range errs {
				fmt.Printf("    %v\n", err)
			}
		}
	},
}

func init() {
	StopCmd.Flags().BoolVar(&allRuns, "all", false, "Stop all runs")
	StopCmd.Flags().StringVar(&runID, "run", "", "Stop a specific run")
	StopCmd.Flags().StringVar(&nodesFlag, "nodes", "", "A comma-separated list of nodes to stop. If a node is a taskgroup (connected to a file-splitter), all tasks in the taskgroup will be stopped unless a --child flag is provided")
	StopCmd.Flags().StringVar(&childrenFlag, "child", "", "A comma-separated list of child tasks to stop. Example: \"--nodes python-script-1 --child 1,2,3\" will stop the first three tasks in the python-script-1 node's taskgroup")
}

func getRelevantRuns(workflow types.Workflow, allRuns bool, runID string, workflowURL string) ([]types.Run, error) {
	switch {
	case allRuns:
		return util.GetRuns(workflow.ID, math.MaxInt, "RUNNING"), nil

	case runID != "":
		runUUID, err := uuid.Parse(runID)
		if err != nil {
			return nil, fmt.Errorf("invalid run ID: %s", runID)
		}
		run := util.GetRunByID(runUUID)
		if run.Status == "RUNNING" {
			return []types.Run{*run}, nil
		} else {
			return nil, fmt.Errorf("run %s is not currently running. Current status: %s", runID, run.Status)
		}

	default:
		workflowURLRunID, _ := util.GetRunIDFromWorkflowURL(workflowURL)
		if runUUID, err := uuid.Parse(workflowURLRunID); err == nil {
			run := util.GetRunByID(runUUID)
			if run.Status == "RUNNING" {
				return []types.Run{*run}, nil
			} else {
				return nil, fmt.Errorf("run %s is not currently running. Current status: %s", workflowURLRunID, run.Status)
			}
		}
		return util.GetRuns(workflow.ID, 1, "RUNNING"), nil
	}
}

func stopRun(run types.Run) error {
	urlReq := fmt.Sprintf("execution/%s/stop/", run.ID.String())
	resp := request.Trickest.Post().DoF(urlReq)
	if resp == nil {
		return fmt.Errorf("failed to stop run %s", run.ID)
	}

	if resp.Status() != http.StatusAccepted {
		return fmt.Errorf("unexpected status code %d while stopping run %s", resp.Status(), run.ID)
	}

	return nil
}

func stopSubJob(subJob types.SubJob) error {
	urlReq := fmt.Sprintf("subjob/%s/stop/", subJob.ID.String())
	resp := request.Trickest.Post().DoF(urlReq)
	if resp == nil {
		return fmt.Errorf("failed to stop node %s (%s)", subJob.Label, subJob.ID)
	}

	if resp.Status() != http.StatusOK {
		return fmt.Errorf("unexpected status code %d while stopping node %s (%s)", resp.Status(), subJob.Label, subJob.ID)
	}

	return nil
}

func stopSubJobChild(run types.Run, node string, child string) error {
	fmt.Printf("Mock for stopping child %s in node %s in run %s\n", child, node, run.ID)
	return nil
}
