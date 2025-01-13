package execute

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/trickest/trickest-cli/cmd/output"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"

	"github.com/google/uuid"
	"github.com/gosuri/uilive"
	"github.com/xlab/treeprint"
)

func WatchRun(runID uuid.UUID, downloadPath string, nodesToDownload []string, filesToDownload []string, timestampOnly bool, machines *types.Machines, showParameters bool) {
	const fmtStr = "%-12s %v\n"
	writer := uilive.New()
	writer.Start()
	defer writer.Stop()

	mutex := &sync.Mutex{}

	if !timestampOnly {
		go func() {
			defer mutex.Unlock()
			signalChannel := make(chan os.Signal, 1)
			signal.Notify(signalChannel, os.Interrupt)
			<-signalChannel

			mutex.Lock()
			_ = writer.Flush()
			writer.Stop()

			if ci {
				stopRun(runID)
				os.Exit(0)
			} else {
				fmt.Println("The program will exit. Would you like to stop the remote execution? (Y/N)")
				var answer string
				for {
					_, _ = fmt.Scan(&answer)
					if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
						stopRun(runID)
						os.Exit(0)
					} else if strings.ToLower(answer) == "n" || strings.ToLower(answer) == "no" {
						os.Exit(0)
					}
				}
			}
		}()
	}

	for {
		mutex.Lock()
		run := GetRunByID(runID)
		if run == nil {
			mutex.Unlock()
			break
		}
		version := output.GetWorkflowVersionByID(*run.WorkflowVersionInfo, uuid.Nil)
		allNodes, roots := CreateTrees(version, false)

		out := ""
		out += fmt.Sprintf(fmtStr, "Name:", run.WorkflowName)
		out += fmt.Sprintf(fmtStr, "Status:", strings.ToLower(run.Status))
		out += fmt.Sprintf(fmtStr, "Machines:", FormatMachines(*machines, true))
		out += fmt.Sprintf(fmtStr, "Created:", run.CreatedDate.In(time.Local).Format(time.RFC1123)+
			" ("+util.FormatDuration(time.Since(run.CreatedDate))+" ago)")
		if run.Status != "PENDING" {
			if !run.StartedDate.IsZero() {
				out += fmt.Sprintf(fmtStr, "Started:", run.StartedDate.In(time.Local).Format(time.RFC1123)+
					" ("+util.FormatDuration(time.Since(run.StartedDate))+" ago)")
			}
		}
		if run.Finished {
			if !run.CompletedDate.IsZero() {
				out += fmt.Sprintf(fmtStr, "Finished:", run.CompletedDate.In(time.Local).Format(time.RFC1123)+
					" ("+util.FormatDuration(time.Since(run.CompletedDate))+" ago)")
			}
			out += fmt.Sprintf(fmtStr, "Duration:", util.FormatDuration(run.CompletedDate.Sub(run.StartedDate)))
		}
		if run.Status == "RUNNING" {
			out += fmt.Sprintf(fmtStr, "Duration:", util.FormatDuration(time.Since(run.StartedDate)))
		}

		subJobs := GetSubJobs(runID)
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

		trees := PrintTrees(roots, &allNodes, showParameters, true)
		out += "\n" + trees
		_, _ = fmt.Fprintln(writer, out)
		_ = writer.Flush()

		if timestampOnly {
			return
		}

		if run.Status == "COMPLETED" || run.Status == "STOPPED" || run.Status == "STOPPING" || run.Status == "FAILED" {
			if downloadPath == "" {
				downloadPath = run.SpaceName
				if run.ProjectName != "" {
					downloadPath += "/" + run.ProjectName
				}
				downloadPath += "/" + run.WorkflowName
			}
			if downloadAllNodes {
				// DownloadRunOutputs downloads all outputs if no nodes were specified
				output.DownloadRunOutput(run, nil, nil, downloadPath)
			} else if len(nodesToDownload) > 0 {
				output.DownloadRunOutput(run, nodesToDownload, filesToDownload, downloadPath)
			}
			mutex.Unlock()
			return
		}
		mutex.Unlock()
	}
}

func PrintTrees(roots []*types.TreeNode, allNodes *map[string]*types.TreeNode, showParameters bool, table bool) string {
	trees := ""
	for _, root := range roots {
		tree := printTree(root, nil, allNodes, showParameters)

		for _, node := range *allNodes {
			node.Printed = false
		}

		if !table {
			trees += tree
			continue
		}

		writerBuffer := new(bytes.Buffer)
		w := tabwriter.NewWriter(writerBuffer, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintf(w, "\tNODE\t STATUS\t DURATION\t OUTPUT\n")

		treeSplit := strings.Split(tree, "\n")
		for _, line := range treeSplit {
			if line != "" {
				if match, _ := regexp.MatchString(`\([-a-z0-9]+-[0-9]+\)`, line); match {
					lineSplit := strings.Split(line, "(")
					nodeName := strings.Trim(lineSplit[1], ")")
					node := (*allNodes)[nodeName]
					_, _ = fmt.Fprintf(w, "\t"+line+"\t"+node.Status+"\t"+
						util.FormatDuration(node.Duration)+"\t"+node.OutputStatus+"\n")
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

func printTree(node *types.TreeNode, branch *treeprint.Tree, allNodes *map[string]*types.TreeNode, showParameters bool) string {
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
	}

	printValue := prefixSymbol + node.Label + " (" + node.Name + ")"
	if branch == nil {
		tree := treeprint.NewWithRoot(printValue)
		branch = &tree
	} else {
		childBranch := (*branch).AddBranch(printValue)
		branch = &childBranch
	}

	if showParameters {
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
							fmt.Println(v)
							v = getNodeNameFromConnectionID(v)
						}
					}
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
		if !(*allNodes)[node.Name].Printed {
			printTree(child, branch, allNodes, showParameters)
		}
	}

	(*allNodes)[node.Name].Printed = true

	return (*branch).String()
}

func CreateTrees(wfVersion *types.WorkflowVersionDetailed, includePrimitiveNodes bool) (map[string]*types.TreeNode, []*types.TreeNode) {
	allNodes = make(map[string]*types.TreeNode, 0)
	roots = make([]*types.TreeNode, 0)

	for _, node := range wfVersion.Data.Nodes {
		allNodes[node.Name] = &types.TreeNode{
			Name:         node.Name,
			Label:        node.Meta.Label,
			Inputs:       &node.Inputs,
			Status:       "pending",
			OutputStatus: "no outputs",
			Children:     make([]*types.TreeNode, 0),
			Parents:      make([]*types.TreeNode, 0),
		}
	}

	if includePrimitiveNodes {
		for _, node := range wfVersion.Data.PrimitiveNodes {
			allNodes[node.Name] = &types.TreeNode{
				Name:  node.Name,
				Label: node.Label,
			}
		}
	}

	for node := range wfVersion.Data.Nodes {
		for _, connection := range wfVersion.Data.Connections {
			if node == getNodeNameFromConnectionID(connection.Destination.ID) {
				child := getNodeNameFromConnectionID(connection.Source.ID)
				if childNode, exists := allNodes[child]; exists {
					if childNode.Parents == nil {
						childNode.Parents = make([]*types.TreeNode, 0)
					}
					childNode.Parents = append(childNode.Parents, allNodes[node])
					allNodes[node].Children = append(allNodes[node].Children, childNode)
				}
			}
		}
	}

	for node := range wfVersion.Data.Nodes {
		if allNodes[node].Parents == nil || len(allNodes[node].Parents) == 0 {
			roots = append(roots, allNodes[node])
		}
	}

	return allNodes, roots
}
