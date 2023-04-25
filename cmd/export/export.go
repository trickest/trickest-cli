package export

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/trickest/trickest-cli/cmd/execute"
	"github.com/trickest/trickest-cli/cmd/list"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type workflowExport struct {
	Name     string       `yaml:"name"`
	Category *string      `yaml:"category,omitempty"`
	Steps    []nodeExport `yaml:"steps"`
}

type nodeExport struct {
	Name    string
	ID      string
	Script  string `yaml:"script,omitempty"`
	Machine string
	Inputs  interface{}
}

type scriptSplitterInputs map[string][]string

var destinationPath string

// ExportCmd represents the export command
var ExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Exports a workflow to a YAML file",
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

		_, _, workflow, found := list.ResolveObjectPath(path, false, false)
		if !found {
			return
		}

		if destinationPath == "" {
			destinationPath = workflow.Name + ".yaml"
		}

		createYAML(workflow, destinationPath)
	},
}

func init() {
	ExportCmd.Flags().StringVarP(&destinationPath, "output", "o", "",
		"Output file (default workflow-name.yaml)")
}

func sortNodes(nodes map[string]*types.Node) []*types.Node {
	var nodesArray []*types.Node
	for name, node := range nodes {
		n := node
		node.Name = name
		nodesArray = append(nodesArray, n)
	}
	sort.Slice(nodesArray, func(i, j int) bool {
		return nodesArray[i].Meta.Coordinates.X < nodesArray[j].Meta.Coordinates.X
	})
	return nodesArray
}

func createYAML(workflow *types.Workflow, destinationPath string) {
	w := workflowExport{
		Steps: make([]nodeExport, 0),
	}
	if workflow.WorkflowCategory != nil {
		w.Category = &workflow.WorkflowCategory.Name
	}
	w.Name = workflow.Name
	version := execute.GetLatestWorkflowVersion(workflow.ID)
	nodes := sortNodes(version.Data.Nodes)
	for _, n := range nodes {
		if n.Type == "TOOL" {
			inputs := make(map[string]interface{})
			for name, input := range n.Inputs {
				if input.Value == nil {
					continue
				}

				inputValueStr := fmt.Sprintf("%v", input.Value)
				if strings.HasPrefix(inputValueStr, "in/file-splitter-") || strings.HasPrefix(inputValueStr, "in/split-to-string-") {
					// in/file-splitter-x:item
					value := strings.Split(inputValueStr, "/")[1]
					value = strings.Split(value, ":")[0]
					inputs[name] = value
				} else if input.Type == "FILE" || input.Type == "FOLDER" {
					value := strings.Split(inputValueStr, "/")[1]
					if strings.HasPrefix(value, "http-input-") || strings.HasPrefix(value, "git-input-") {
						if v, ok := version.Data.PrimitiveNodes[value]; ok {
							value = v.Value.(string)
						}
					}
					inputs[name] = value
				} else {
					if _, ok := input.Value.(bool); ok {
						inputs[name] = input.Value
					} else {
						inputs[name] = inputValueStr
					}
				}
			}
			w.Steps = append(w.Steps, nodeExport{
				Name:    n.Meta.Label,
				ID:      n.Name,
				Machine: n.BeeType,
				Inputs:  inputs,
			})
		} else if n.Type == "SCRIPT" {
			inputs := scriptSplitterInputs{
				"file":   make([]string, 0),
				"folder": make([]string, 0),
			}
			for _, input := range n.Inputs {
				if input.Value != nil {
					value := strings.Split(input.Value.(string), "/")[1]
					if v, ok := version.Data.PrimitiveNodes[value]; ok {
						value = v.Value.(string)
					}
					if input.Type == "FILE" {
						inputs["file"] = append(inputs["file"], value)
					}
					if input.Type == "FOLDER" {
						inputs["folder"] = append(inputs["folder"], value)
					}
				}
			}
			if len(inputs["file"]) == 0 {
				delete(inputs, "file")
			}
			if len(inputs["folder"]) == 0 {
				delete(inputs, "folder")
			}
			w.Steps = append(w.Steps, nodeExport{
				Name:    n.Meta.Label,
				ID:      n.Name,
				Script:  n.Script.Source,
				Machine: n.BeeType,
				Inputs:  inputs,
			})
		} else if n.Type == "SPLITTER" {
			inputs := make([]string, 0)
			for n, node := range n.Inputs {
				if n != "multiple" {
					name := strings.Split(n, "/")
					if node.Type == "FILE" {
						inputs = append(inputs, name[1])
					}
					if node.Type == "FOLDER" {
						inputs = append(inputs, name[1])
					}
				}
			}
			w.Steps = append(w.Steps, nodeExport{
				Name:    n.Meta.Label,
				ID:      n.Name,
				Machine: n.BeeType,
				Inputs:  inputs,
			})
		}
	}

	yamlData, err := yaml.Marshal(&w)
	if err != nil {
		fmt.Println("Couldn't marshal workflow to YAML")
		os.Exit(0)
	}

	file, err := os.Create(destinationPath)
	if err != nil {
		fmt.Println("Couldn't create/overwrite file: " + destinationPath)
		os.Exit(0)
	}
	defer file.Close()

	_, err = file.Write(yamlData)
	if err != nil {
		fmt.Println("Couldn't write data to file: " + destinationPath)
		os.Exit(0)
	}

	path, err := filepath.Abs(file.Name())
	if err != nil {
		return
	}
	fmt.Println("Workflow \"" + workflow.Name + "\" successfully exported to " + path)
}
