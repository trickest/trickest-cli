package display

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/hako/durafmt"
	"github.com/trickest/trickest-cli/pkg/stats"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/xlab/treeprint"
)

// TreeNode represents a node in the workflow tree
type TreeNode struct {
	Name         string
	Label        string
	Inputs       *map[string]*trickest.NodeInput
	Status       string
	OutputStatus string
	Duration     time.Duration
	Printed      bool
	Children     []*TreeNode
	Parents      []*TreeNode

	TaskGroup      bool
	TaskGroupStats stats.TaskGroupStats
}

// RunPrinter handles the formatting and display of run information
type RunPrinter struct {
	includePrimitiveNodes bool
	writer                io.Writer
}

// NewRunPrinter creates a new RunPrinter instance
func NewRunPrinter(includePrimitiveNodes bool, writer io.Writer) *RunPrinter {
	if writer == nil {
		writer = os.Stdout
	}
	return &RunPrinter{
		includePrimitiveNodes: includePrimitiveNodes,
		writer:                writer,
	}
}

// PrintAll formats and prints run details and subjob tree
func (p *RunPrinter) PrintAll(run *trickest.Run, subJobs []trickest.SubJob, version *trickest.WorkflowVersion, includeTaskGroupStats bool) {
	var output strings.Builder

	// Print basic run details
	output.WriteString(p.formatKeyValue("Name", run.WorkflowName))
	output.WriteString(p.formatKeyValue("Status", run.Status))
	output.WriteString(p.formatKeyValue("Machines", formatMachines(run.Machines)))
	output.WriteString(p.formatKeyValue("Fleet", run.FleetName))
	output.WriteString("\n")

	// Print timestamps
	if run.CreatedDate != nil {
		output.WriteString(p.formatKeyValue("Created",
			run.CreatedDate.In(time.Local).Format(time.RFC1123)+" ("+FormatDuration(time.Since(*run.CreatedDate))+" ago)"))
	}
	if run.Status != "PENDING" {
		if run.StartedDate != nil {
			output.WriteString(p.formatKeyValue("Started",
				run.StartedDate.In(time.Local).Format(time.RFC1123)+" ("+FormatDuration(time.Since(*run.StartedDate))+" ago)"))
		}
	}
	if run.Finished {
		output.WriteString(p.formatKeyValue("Finished",
			run.CompletedDate.In(time.Local).Format(time.RFC1123)+" ("+FormatDuration(time.Since(*run.CompletedDate))+" ago)"))
	}
	output.WriteString("\n")

	// Print duration and average duration
	if run.Finished {
		output.WriteString(p.formatKeyValue("Duration", FormatDuration(run.CompletedDate.Sub(*run.StartedDate))))
	} else if run.Status == "RUNNING" {
		output.WriteString(p.formatKeyValue("Duration", FormatDuration(time.Since(*run.StartedDate))))
	}
	if run.AverageDuration != nil {
		output.WriteString(p.formatKeyValue("Average Duration", FormatDuration(run.AverageDuration.Duration)))
	}
	output.WriteString("\n")

	// Print subjob insights
	if run.RunInsights != nil {
		output.WriteString(p.formatKeyValue("Total Jobs", fmt.Sprintf("%d", run.RunInsights.Total)))
		output.WriteString(p.formatSubJobStatus("Succeeded", run.RunInsights.Succeeded, run.RunInsights.Total))
		output.WriteString(p.formatSubJobStatus("Running", run.RunInsights.Running, run.RunInsights.Total))
		output.WriteString(p.formatSubJobStatus("Pending", run.RunInsights.Pending, run.RunInsights.Total))
		output.WriteString(p.formatSubJobStatus("Failed", run.RunInsights.Failed, run.RunInsights.Total))
		output.WriteString(p.formatSubJobStatus("Stopping", run.RunInsights.Stopping, run.RunInsights.Total))
		output.WriteString(p.formatSubJobStatus("Stopped", run.RunInsights.Stopped, run.RunInsights.Total))
		output.WriteString("\n")
	}

	// Print subjob tree
	//
	// For nodes without an associated subjob (i.e. not executed):
	// - If the run is still running, mark them as "pending"
	// - If the run is finished, mark them as "stopped"
	defaultSubJobStatus := "stopped"
	if run.Status == "RUNNING" {
		defaultSubJobStatus = "pending"
	}
	output.WriteString(p.formatSubJobTree(subJobs, version, defaultSubJobStatus, includeTaskGroupStats))

	fmt.Fprint(p.writer, output.String())
}

// formatMachines formats the machine allocation for the run
func formatMachines(machines trickest.Machines) string {
	if machines.Default != nil && *machines.Default > 0 {
		return fmt.Sprintf("%d", *machines.Default)
	}
	if machines.SelfHosted != nil && *machines.SelfHosted > 0 {
		return fmt.Sprintf("%d", *machines.SelfHosted)
	}
	return ""
}

// formatKeyValue formats a key-value pair with a fixed width
func (p *RunPrinter) formatKeyValue(key, value string) string {
	return fmt.Sprintf("%-18s %v\n", key+":", value)
}

// FormatDuration formats a duration for printing
func FormatDuration(duration time.Duration) string {
	duration = duration.Round(time.Second)
	units := durafmt.Units{
		Year:   durafmt.Unit{Singular: "year", Plural: "years"},
		Week:   durafmt.Unit{Singular: "week", Plural: "weeks"},
		Day:    durafmt.Unit{Singular: "day", Plural: "days"},
		Hour:   durafmt.Unit{Singular: "h", Plural: "h"},
		Minute: durafmt.Unit{Singular: "m", Plural: "m"},
		Second: durafmt.Unit{Singular: "s", Plural: "s"},
	}

	str := durafmt.Parse(duration).LimitFirstN(2).Format(units)
	str = strings.Replace(str, " s", "s", 1)
	str = strings.Replace(str, " m", "m", 1)
	str = strings.Replace(str, " h", "h", 1)

	return str
}

// formatSubJobTree formats the subjob tree for a workflow run
func (p *RunPrinter) formatSubJobTree(subJobs []trickest.SubJob, version *trickest.WorkflowVersion, defaultSubJobStatus string, includeTaskGroupStats bool) string {
	allNodes, roots := p.createTrees(subJobs, version, defaultSubJobStatus, includeTaskGroupStats)
	return p.printTrees(roots, &allNodes, includeTaskGroupStats)
}

func (p *RunPrinter) createTrees(subJobs []trickest.SubJob, wfVersion *trickest.WorkflowVersion, defaultSubJobStatus string, includeTaskGroupStats bool) (map[string]*TreeNode, []*TreeNode) {
	allNodes := make(map[string]*TreeNode, 0)
	roots := make([]*TreeNode, 0)

	for _, node := range wfVersion.Data.Nodes {
		allNodes[node.Name] = &TreeNode{
			Name:         node.Name,
			Label:        node.Meta.Label,
			Inputs:       &node.Inputs,
			Status:       defaultSubJobStatus,
			OutputStatus: "no outputs",
			Children:     make([]*TreeNode, 0),
			Parents:      make([]*TreeNode, 0),
		}
	}

	if p.includePrimitiveNodes {
		for _, node := range wfVersion.Data.PrimitiveNodes {
			allNodes[node.Name] = &TreeNode{
				Name:  node.Name,
				Label: node.Label,
			}
		}
	}

	for node := range wfVersion.Data.Nodes {
		for _, connection := range wfVersion.Data.Connections {

			connectionDestination, err := getNodeNameFromConnectionID(connection.Destination.ID)
			if err != nil {
				fmt.Println("Error getting node name from connection ID:", err)
				continue
			}
			if node == connectionDestination {
				connectionSource, err := getNodeNameFromConnectionID(connection.Source.ID)
				if err != nil {
					fmt.Println("Error getting node name from connection ID:", err)
					continue
				}
				if childNode, exists := allNodes[connectionSource]; exists {
					if childNode.Parents == nil {
						childNode.Parents = make([]*TreeNode, 0)
					}
					childNode.Parents = append(childNode.Parents, allNodes[node])
					allNodes[node].Children = append(allNodes[node].Children, childNode)
				}
			}
		}
	}

	for node := range wfVersion.Data.Nodes {
		if len(allNodes[node].Parents) == 0 {
			roots = append(roots, allNodes[node])
		}
	}

	for _, sj := range subJobs {
		allNodes[sj.Name].Status = strings.ToLower(sj.Status)
		allNodes[sj.Name].OutputStatus = strings.ReplaceAll(strings.ToLower(sj.OutputsStatus), "_", " ")
		if sj.Finished {
			allNodes[sj.Name].Duration = sj.FinishedDate.Sub(sj.StartedDate).Round(time.Second)
		} else if sj.StartedDate.IsZero() {
			allNodes[sj.Name].Duration = *new(time.Duration)
		} else {
			allNodes[sj.Name].Duration = time.Since(sj.StartedDate).Round(time.Second)
		}
		if sj.TaskGroup && includeTaskGroupStats {
			allNodes[sj.Name].TaskGroup = true
			allNodes[sj.Name].TaskGroupStats = stats.CalculateTaskGroupStats(sj)
		}
	}

	return allNodes, roots
}

func (p *RunPrinter) formatSubJobStatus(status string, count int, total int) string {
	if count == 0 {
		return ""
	}
	return p.formatKeyValue(status, fmt.Sprintf("%d/%d (%.2f%%)", count, total, float64(count)/float64(total)*100))
}

func getNodeNameFromConnectionID(id string) (string, error) {
	idSplit := strings.Split(id, "/")
	if len(idSplit) < 3 {
		return "", fmt.Errorf("invalid connection ID format: %s", id)
	}
	return idSplit[1], nil
}

func (p *RunPrinter) printTrees(roots []*TreeNode, allNodes *map[string]*TreeNode, includeTaskGroupStats bool) string {
	trees := ""
	nodePattern := regexp.MustCompile(`\([-a-z0-9]+-[0-9]+\)`)

	for _, root := range roots {
		tree := p.printTree(root, nil, allNodes, includeTaskGroupStats)

		for _, node := range *allNodes {
			node.Printed = false
		}

		writerBuffer := new(bytes.Buffer)
		w := tabwriter.NewWriter(writerBuffer, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintf(w, "\tNODE\t STATUS\t DURATION\t OUTPUT\n")

		treeSplit := strings.Split(tree, "\n")
		for _, line := range treeSplit {
			if line != "" {
				if nodePattern.MatchString(line) {
					lineSplit := strings.Split(line, "(")
					nodeName := strings.Trim(lineSplit[1], ")")
					node := (*allNodes)[nodeName]
					_, _ = fmt.Fprintf(w, "\t"+line+"\t"+node.Status+"\t"+
						FormatDuration(node.Duration)+"\t"+node.OutputStatus+"\n")
				} else {
					_, _ = fmt.Fprintf(w, "\t"+line+"\t\t\t\n")
				}
			}
		}
		_ = w.Flush()
		trees += writerBuffer.String()
	}

	return trees
}

func (p *RunPrinter) printTree(node *TreeNode, branch *treeprint.Tree, allNodes *map[string]*TreeNode, includeTaskGroupStats bool) string {
	prefixSymbol := ""
	switch node.Status {
	case "pending":
		prefixSymbol = "\u23f3 " //⏳
	case "running":
		prefixSymbol = "\U0001f535 " //🔵
	case "succeeded":
		prefixSymbol = "\u2705 " //✅
	case "error", "failed":
		prefixSymbol = "\u274c " //❌
	case "stopped", "stopping":
		prefixSymbol = "\u26d4 " //⛔
	}

	printValue := prefixSymbol + node.Label + " (" + node.Name + ")"
	if branch == nil {
		tree := treeprint.NewWithRoot(printValue)
		branch = &tree
	} else {
		childBranch := (*branch).AddBranch(printValue)
		branch = &childBranch
	}

	if node.TaskGroup && includeTaskGroupStats {
		taskInfo := (*branch).AddBranch("Task Group Info")
		tasksBranch := taskInfo.AddBranch(fmt.Sprintf("%d tasks", node.TaskGroupStats.Count))
		if node.TaskGroupStats.Status.Succeeded != node.TaskGroupStats.Count { // Only show task group detailed counts if not all tasks succeeded
			// %%%% is a double-escaped percent sign, once for the branch, once for the sprintf
			if node.TaskGroupStats.Status.Succeeded > 0 {
				tasksBranch.AddBranch(fmt.Sprintf("%d succeeded (%.2f%%%%)", node.TaskGroupStats.Status.Succeeded, float64(node.TaskGroupStats.Status.Succeeded)/float64(node.TaskGroupStats.Count)*100))
			}
			if node.TaskGroupStats.Status.Running > 0 {
				tasksBranch.AddBranch(fmt.Sprintf("%d running (%.2f%%%%)", node.TaskGroupStats.Status.Running, float64(node.TaskGroupStats.Status.Running)/float64(node.TaskGroupStats.Count)*100))
			}
			if node.TaskGroupStats.Status.Pending > 0 {
				tasksBranch.AddBranch(fmt.Sprintf("%d pending (%.2f%%%%)", node.TaskGroupStats.Status.Pending, float64(node.TaskGroupStats.Status.Pending)/float64(node.TaskGroupStats.Count)*100))
			}
			if node.TaskGroupStats.Status.Failed > 0 {
				tasksBranch.AddBranch(fmt.Sprintf("%d failed (%.2f%%%%)", node.TaskGroupStats.Status.Failed, float64(node.TaskGroupStats.Status.Failed)/float64(node.TaskGroupStats.Count)*100))
			}
			if node.TaskGroupStats.Status.Stopping > 0 {
				tasksBranch.AddBranch(fmt.Sprintf("%d stopping (%.2f%%%%)", node.TaskGroupStats.Status.Stopping, float64(node.TaskGroupStats.Status.Stopping)/float64(node.TaskGroupStats.Count)*100))
			}
			if node.TaskGroupStats.Status.Stopped > 0 {
				tasksBranch.AddBranch(fmt.Sprintf("%d stopped (%.2f%%%%)", node.TaskGroupStats.Status.Stopped, float64(node.TaskGroupStats.Status.Stopped)/float64(node.TaskGroupStats.Count)*100))
			}
		}

		if hasInterestingStats(node.Name) {
			durationBranch := taskInfo.AddBranch("Task Duration Stats")
			if node.TaskGroupStats.MaxDuration.Duration > 0 && node.TaskGroupStats.MaxDuration.TaskIndex != -1 {
				durationBranch.AddNode(fmt.Sprintf("Max: %s (task %d)", FormatDuration(node.TaskGroupStats.MaxDuration.Duration), node.TaskGroupStats.MaxDuration.TaskIndex))
			}
			if node.TaskGroupStats.MinDuration.Duration > 0 && node.TaskGroupStats.MinDuration.TaskIndex != -1 {
				durationBranch.AddNode(fmt.Sprintf("Min: %s (task %d)", FormatDuration(node.TaskGroupStats.MinDuration.Duration), node.TaskGroupStats.MinDuration.TaskIndex))
			}

			if node.TaskGroupStats.Median > 0 {
				medianBranch := durationBranch.AddBranch(fmt.Sprintf("Median: %s", FormatDuration(node.TaskGroupStats.Median)))
				if node.TaskGroupStats.MedianAbsoluteDeviation > 0 {
					medianBranch.AddNode(fmt.Sprintf("Median Absolute Deviation: %s", FormatDuration(node.TaskGroupStats.MedianAbsoluteDeviation)))
				}
			}
			if len(node.TaskGroupStats.Outliers) > 0 {
				outlierBranch := durationBranch.AddBranch("Outliers")
				for _, outlier := range node.TaskGroupStats.Outliers {
					if outlier.Duration < node.TaskGroupStats.Median {
						timeDiff := node.TaskGroupStats.Median - outlier.Duration
						outlierBranch.AddNode(fmt.Sprintf("Task %d: %s faster than median (duration: %s)", outlier.TaskIndex, FormatDuration(timeDiff), FormatDuration(outlier.Duration)))
					} else {
						timeDiff := outlier.Duration - node.TaskGroupStats.Median
						outlierBranch.AddNode(fmt.Sprintf("Task %d: %s slower than median (duration: %s)", outlier.TaskIndex, FormatDuration(timeDiff), FormatDuration(outlier.Duration)))
					}
				}
			}
		}
	}

	if p.includePrimitiveNodes && node.Inputs != nil {
		inputNames := make([]string, 0)
		for input := range *node.Inputs {
			inputNames = append(inputNames, input)
		}
		sort.Strings(inputNames)
		parameters := (*branch).AddBranch("parameters")
		for _, inputName := range inputNames {
			input := (*node.Inputs)[inputName]
			param := inputName + ": "
			if input.Value != nil {
				switch v := input.Value.(type) {
				case string:
					if strings.HasPrefix(v, "in/") {
						if strings.Contains(v, "/file-splitter-") || strings.Contains(v, "/split-to-string-") {
							v = strings.TrimPrefix(v, "/in")
							v = strings.TrimSuffix(v, ":item")
						} else {
							nodeName, err := getNodeNameFromConnectionID(v)
							if err != nil {
								fmt.Println("Error getting node name:", err)
								continue
							}
							if primitiveNode, exists := (*allNodes)[nodeName]; exists && primitiveNode.Inputs == nil {
								v = primitiveNode.Label
							} else {
								v = nodeName
							}
						}
					}
					v = strings.ReplaceAll(v, "%", "%%")
					if strings.HasPrefix(param, "file/") || strings.HasPrefix(param, "folder/") {
						parameters.AddNode(v)
					} else {
						parameters.AddNode(param + v)
					}
				case int:
					parameters.AddNode(param + strconv.Itoa(v))
				case bool:
					parameters.AddNode(param + strconv.FormatBool(v))
				}
			}
		}
	}

	for _, child := range node.Children {
		if !(*allNodes)[node.Name].Printed && (*allNodes)[child.Name].Inputs != nil {
			p.printTree(child, branch, allNodes, includeTaskGroupStats)
		}
	}

	(*allNodes)[node.Name].Printed = true

	return (*branch).String()
}

// hasInterestingStats returns true if the node's stats are worth displaying
func hasInterestingStats(nodeName string) bool {
	unInterestingNodeNames := map[string]bool{
		"batch-output":   true,
		"string-to-file": true,
	}

	parts := strings.Split(nodeName, "-")
	if len(parts) < 2 {
		return true
	}
	baseNodeName := strings.Join(parts[:len(parts)-1], "-")
	return !unInterestingNodeNames[baseNodeName]
}
