package execute

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/trickest/trickest-cli/cmd/create"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"

	"github.com/google/uuid"

	"github.com/spf13/cobra"
)

var (
	newWorkflowName      string
	configFile           string
	watch                bool
	showParams           bool
	executionMachines    types.Machines
	fleet                *types.Fleet
	nodesToDownload      []string
	allNodes             map[string]*types.TreeNode
	roots                []*types.TreeNode
	maxMachines          bool
	machineConfiguration string
	downloadAllNodes     bool
	outputsDirectory     string
	outputNodesFlag      string
	ci                   bool
	createProject        bool
	fleetName            string
	useStaticIPs         bool
)

// ExecuteCmd represents the execute command
var ExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute a workflow",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if util.SpaceName == "" {
			util.SpaceName = "Playground"
		}
		path := util.FormatPath()
		if path == "" {
			if len(args) == 0 {
				fmt.Println("Workflow name or path must be specified!")
				return
			}
			path = strings.Trim(args[0], "/")
		} else {
			if len(args) > 0 {
				fmt.Println("Please use either path or flag syntax for the platform objects.")
				return
			}
		}

		if useStaticIPs && !util.VaultHasStaticIPs() {
			fmt.Println("The Static IP Addresses feature is not enabled for your account. Please contact support.")
			return
		}

		fleet = util.GetFleetInfo(fleetName)
		if fleet == nil {
			return
		}
		version := prepareForExec(path)
		if version == nil {
			fmt.Println("Couldn't find or create the workflow version!")
			os.Exit(0)
		}

		allNodes, roots = CreateTrees(version, false)

		if executionMachines == (types.Machines{}) {
			if maxMachines {
				executionMachines = version.MaxMachines
			} else if machineConfiguration != "" {
				machines, err := parseMachineConfiguration(machineConfiguration)
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}

				if len(fleet.Machines) == 3 {
					// 3 types of machines: small, medium, and large
					executionMachines = machines

					if machines.Default != nil {
						fmt.Printf("Error: you need to use the small-medium-large format to specify the numbers of machines (e.g. 1-2-3)")
						os.Exit(1)
					}
				} else {
					// 1 type of machine
					executionMachines, err = handleSingleMachineType(*fleet, machines)
					if err != nil {
						fmt.Printf("Error: %s\n", err)
						os.Exit(1)
					}
				}
			} else {
				executionMachines = setMachinesToMinimum(version.MaxMachines)
			}
		}

		outputNodes := make([]string, 0)
		if outputNodesFlag != "" {
			outputNodes = strings.Split(outputNodesFlag, ",")
		}

		if !maxMachinesTypeCompatible(executionMachines, version.MaxMachines) {
			fmt.Println("Workflow maximum machines types are not compatible with config machines!")
			fmt.Println("Workflow max machines: " + FormatMachines(version.MaxMachines, true))
			fmt.Println("Config machines: " + FormatMachines(executionMachines, true))
			os.Exit(0)
		}

		createRun(version.ID, fleet.ID, watch, outputNodes, outputsDirectory, useStaticIPs)
	},
}

func parseMachineConfiguration(config string) (types.Machines, error) {
	pattern := `^\d+-\d+-\d+$`
	regex := regexp.MustCompile(pattern)

	if regex.MatchString(config) {
		// 3 types of machines, 3 hyphen-delimited inputs
		parts := strings.Split(config, "-")

		if len(parts) != 3 {
			return types.Machines{}, fmt.Errorf("invalid number of machines in machine configuration \"%s\"", config)
		}

		sizes := make([]int, 3)
		for index, part := range parts {
			if size, err := strconv.Atoi(part); err == nil {
				sizes[index] = size
			} else {
				return types.Machines{}, fmt.Errorf("invalid machine configuration \"%s\"", config)
			}
		}

		var machines types.Machines

		if sizes[0] != 0 {
			machines.Small = &sizes[0]
		}

		if sizes[1] != 0 {
			machines.Medium = &sizes[1]
		}

		if sizes[2] != 0 {
			machines.Large = &sizes[2]
		}

		return machines, nil
	}

	// One type of machine
	val, err := strconv.Atoi(config)
	if err != nil {
		return types.Machines{}, fmt.Errorf("invalid machine configuration \"%s\"", config)
	}

	return types.Machines{Default: &val}, nil
}

func handleSingleMachineType(fleet types.Fleet, machines types.Machines) (types.Machines, error) {
	var configMachines types.Machines

	var defaultOrSelfHosted int
	if machines.Default != nil {
		defaultOrSelfHosted = *machines.Default
	} else {
		// Backward-compatibility with the small-medium-large format
		if machines.Small != nil {
			defaultOrSelfHosted += *machines.Small
		}
		if machines.Medium != nil {
			defaultOrSelfHosted += *machines.Medium
		}
		if machines.Large != nil {
			defaultOrSelfHosted += *machines.Large
		}

		fmt.Printf("Warning: You have one type of machine in your fleet. %d identical or self-hosted machines will be used.\n", defaultOrSelfHosted)
	}

	if defaultOrSelfHosted == 0 {
		return types.Machines{}, fmt.Errorf("cannot run the workflow on %d machines", defaultOrSelfHosted)
	}

	if fleet.Type == "MANAGED" {
		configMachines.Default = &defaultOrSelfHosted
	} else if fleet.Type == "HOSTED" {
		configMachines.SelfHosted = &defaultOrSelfHosted
	} else {
		return types.Machines{}, fmt.Errorf("unsupported format. Use small-medium-large (e.g., 0-0-3)")
	}
	return configMachines, nil
}

func init() {
	ExecuteCmd.Flags().StringVar(&newWorkflowName, "set-name", "", "Set workflow name")
	ExecuteCmd.Flags().StringVar(&configFile, "config", "", "YAML file for run configuration")
	ExecuteCmd.Flags().BoolVar(&watch, "watch", false, "Watch the execution running")
	ExecuteCmd.Flags().BoolVar(&showParams, "show-params", false, "Show parameters in the workflow tree")
	ExecuteCmd.Flags().BoolVar(&maxMachines, "max", false, "Use maximum number of machines for workflow execution")
	ExecuteCmd.Flags().StringVar(&machineConfiguration, "machines", "", "Specify the number of machines. Use one value for default/self-hosted machines (--machines 3) or three values for small-medium-large (--machines 1-1-1)")
	ExecuteCmd.Flags().BoolVar(&downloadAllNodes, "output-all", false, "Download all outputs when the execution is finished")
	ExecuteCmd.Flags().StringVar(&outputNodesFlag, "output", "", "A comma separated list of nodes which outputs should be downloaded when the execution is finished")
	ExecuteCmd.Flags().StringVar(&outputsDirectory, "output-dir", "", "Path to directory which should be used to store outputs")
	ExecuteCmd.Flags().BoolVar(&ci, "ci", false, "Run in CI mode (in-progreess executions will be stopped when the CLI is forcefully stopped - if not set, you will be asked for confirmation)")
	ExecuteCmd.Flags().BoolVar(&createProject, "create-project", false, "If the project doesn't exist, create it using the project flag as its name (or workflow name if not set)")
	ExecuteCmd.Flags().StringVar(&fleetName, "fleet", "", "The name of the fleet to use to execute the workflow")
	ExecuteCmd.Flags().BoolVar(&useStaticIPs, "use-static-ips", false, "Use static IP addresses for the execution")
}

func prepareForExec(objectPath string) *types.WorkflowVersionDetailed {
	pathSplit := strings.Split(strings.Trim(objectPath, "/"), "/")
	var wfVersion *types.WorkflowVersionDetailed
	var primitiveNodes map[string]*types.PrimitiveNode
	projectCreated := false

	space, project, workflow, _ := util.ResolveObjectPath(objectPath, true)
	if util.URL != "" {
		space, project, workflow, _ = util.ResolveObjectURL(util.URL)
	}

	if workflow != nil && newWorkflowName == "" {
		// Executing an existing workflow
		wfVersion = GetLatestWorkflowVersion(workflow.ID, fleet.ID)
		if configFile != "" {
			update, updatedWfVersion, newPrimitiveNodes := readConfig(configFile, wfVersion)
			if update {
				uploadFilesIfNeeded(newPrimitiveNodes)
				wfVersion = createNewVersion(updatedWfVersion)
				return wfVersion
			}
		}
	} else {
		// Executing from library
		wfName := pathSplit[len(pathSplit)-1]
		libraryWorkflows := util.GetWorkflows(uuid.Nil, uuid.Nil, wfName, true)
		if libraryWorkflows != nil && len(libraryWorkflows) > 0 {
			// Executing from library
			for _, wf := range libraryWorkflows {
				if strings.ToLower(wf.Name) == strings.ToLower(wfName) {
					if project == nil && createProject {
						projectName := util.ProjectName
						if projectName == "" {
							projectName = wfName
						}
						project = create.CreateProjectIfNotExists(space, projectName)
						projectCreated = true
					}

					if newWorkflowName == "" {
						newWorkflowName = wf.Name
					}
					copyDestination := space.Name
					if project != nil {
						copyDestination += "/" + project.Name
					}
					copyDestination += "/" + newWorkflowName
					fmt.Println("Copying " + wf.Name + " from the library to " + copyDestination)
					projID := uuid.Nil
					if project != nil {
						projID = project.ID
					}
					newWorkflowID := copyWorkflow(space.ID, projID, wf.ID)
					if newWorkflowID == uuid.Nil {
						fmt.Println("Couldn't copy workflow from the library!")
						os.Exit(0)
					}

					newWorkflow := util.GetWorkflowByID(newWorkflowID)
					if newWorkflow.Name != newWorkflowName {
						newWorkflow.Name = newWorkflowName
						updateWorkflow(newWorkflow, projectCreated)
					}

					copiedWfVersion := GetLatestWorkflowVersion(newWorkflow.ID, fleet.ID)
					if copiedWfVersion == nil {
						fmt.Println("No workflow version found for " + newWorkflow.Name)
						os.Exit(0)
					}
					update := false
					var updatedWfVersion *types.WorkflowVersionDetailed
					if configFile != "" {
						update, updatedWfVersion, primitiveNodes = readConfig(configFile, copiedWfVersion)
					}

					if update {
						if updatedWfVersion == nil {
							fmt.Println("Sorry, couldn't update workflow!")
							os.Exit(0)
						}
						uploadFilesIfNeeded(primitiveNodes)
						updatedWfVersion.WorkflowInfo = newWorkflow.ID
						wfVersion = createNewVersion(updatedWfVersion)
					} else {
						copiedWfVersion.WorkflowInfo = newWorkflow.ID
						if len(copiedWfVersion.Data.PrimitiveNodes) > 0 {
							for _, pNode := range copiedWfVersion.Data.PrimitiveNodes {
								pNode.Coordinates.X += 0.00001
								break
							}
						} else if len(copiedWfVersion.Data.Nodes) > 0 {
							for _, node := range copiedWfVersion.Data.Nodes {
								node.Meta.Coordinates.X += 0.00001
								break
							}
						} else {
							fmt.Println("No nodes found in workflow version!")
							os.Exit(0)
						}
						wfVersion = createNewVersion(copiedWfVersion)
					}
					return wfVersion
				}
			}
		} else {
			fmt.Println("Couldn't find a workflow named " + wfName + " in the library!")
			os.Exit(0)
		}
	}

	wfVersion = createNewVersion(wfVersion)
	return wfVersion
}
