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
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/xlab/treeprint"
)

// TreeNode represents a node in the workflow tree
type TreeNode struct {
	Name         string
	Label        string
	Inputs       *map[string]*trickest.NodeInput
	Outputs      *map[string]*trickest.NodeOutput
	Status       string
	OutputStatus string
	Duration     time.Duration
	Printed      bool
	Children     []*TreeNode
	Parents      []*TreeNode
	Coordinates  struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	Type   string
	Script *struct {
		Args   []any  `json:"args"`
		Image  string `json:"image"`
		Source string `json:"source"`
	}
	Container *struct {
		Args    []string `json:"args,omitempty"`
		Image   string   `json:"image"`
		Command []string `json:"command"`
	}
	OutputCommand   *string
	WorkerConnected *string
	Workflow        *string
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
func (p *RunPrinter) PrintAll(run *trickest.Run, subJobs []trickest.SubJob, version *trickest.WorkflowVersion) {
	var output strings.Builder

	// Print basic run details
	output.WriteString(p.formatKeyValue("Name", run.WorkflowName))
	output.WriteString(p.formatKeyValue("Status", run.Status))
	output.WriteString(p.formatKeyValue("Machines", formatMachines(run.Machines)))

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
		output.WriteString(p.formatKeyValue("Duration", FormatDuration(run.CompletedDate.Sub(*run.StartedDate))))
	} else if run.Status == "RUNNING" {
		if run.StartedDate != nil {
			output.WriteString(p.formatKeyValue("Duration", FormatDuration(time.Since(*run.StartedDate))))
		}
	}

	output.WriteString("\n")

	// Print subjob insights
	if run.RunInsights != nil {
		output.WriteString("Subjob Insights:\n")
		output.WriteString(p.formatKeyValue("Total", fmt.Sprintf("%d", run.RunInsights.Total)))
		output.WriteString(p.formatSubJobStatus("Succeeded", run.RunInsights.Succeeded, run.RunInsights.Total))
		output.WriteString(p.formatSubJobStatus("Running", run.RunInsights.Running, run.RunInsights.Total))
		output.WriteString(p.formatSubJobStatus("Pending", run.RunInsights.Pending, run.RunInsights.Total))
		output.WriteString(p.formatSubJobStatus("Failed", run.RunInsights.Failed, run.RunInsights.Total))
		output.WriteString(p.formatSubJobStatus("Stopping", run.RunInsights.Stopping, run.RunInsights.Total))
		output.WriteString(p.formatSubJobStatus("Stopped", run.RunInsights.Stopped, run.RunInsights.Total))
		output.WriteString("\n")
	}

	// Print subjob tree
	output.WriteString(p.formatSubJobTree(subJobs, version))

	fmt.Fprint(p.writer, output.String())
}

// formatMachines formats the machine allocation for the run
func formatMachines(machines trickest.Machines) string {
	machineCounts := map[string]*int{
		"self hosted": machines.SelfHosted,
		"default":     machines.Default,
	}

	var formattedMachines []string
	for machine, count := range machineCounts {
		if formatted := formatMachineCount(machine, count); formatted != "" {
			formattedMachines = append(formattedMachines, formatted)
		}
	}

	separator := ", "
	return strings.Join(formattedMachines, separator)
}

// formatMachineCount formats a machine count for printing
func formatMachineCount(name string, count *int) string {
	if count == nil || *count == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %d", name, *count)
}

// formatKeyValue formats a key-value pair with a fixed width
func (p *RunPrinter) formatKeyValue(key, value string) string {
	return fmt.Sprintf("%-12s %v\n", key+":", value)
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
func (p *RunPrinter) formatSubJobTree(subJobs []trickest.SubJob, version *trickest.WorkflowVersion) string {
	allNodes, roots := p.createTrees(subJobs, version)
	return p.printTrees(roots, &allNodes)
}

func (p *RunPrinter) createTrees(subJobs []trickest.SubJob, wfVersion *trickest.WorkflowVersion) (map[string]*TreeNode, []*TreeNode) {
	allNodes := make(map[string]*TreeNode, 0)
	roots := make([]*TreeNode, 0)

	for _, node := range wfVersion.Data.Nodes {
		allNodes[node.Name] = &TreeNode{
			Name:         node.Name,
			Label:        node.Meta.Label,
			Inputs:       &node.Inputs,
			Status:       "pending",
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

func (p *RunPrinter) printTrees(roots []*TreeNode, allNodes *map[string]*TreeNode) string {
	trees := ""
	nodePattern := regexp.MustCompile(`\([-a-z0-9]+-[0-9]+\)`)

	for _, root := range roots {
		tree := p.printTree(root, nil, allNodes)

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

func (p *RunPrinter) printTree(node *TreeNode, branch *treeprint.Tree, allNodes *map[string]*TreeNode) string {
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
			p.printTree(child, branch, allNodes)
		}
	}

	(*allNodes)[node.Name].Printed = true

	return (*branch).String()
}
