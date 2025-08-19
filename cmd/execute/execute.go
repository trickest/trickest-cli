package execute

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/trickest/trickest-cli/pkg/actions"
	"github.com/trickest/trickest-cli/pkg/config"
	display "github.com/trickest/trickest-cli/pkg/display/run"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/pkg/workflowbuilder"
	"github.com/trickest/trickest-cli/util"

	"github.com/google/uuid"

	"github.com/spf13/cobra"
)

const (
	DefaultFleetName = "Managed fleet"
	DefaultMachines  = 1
)

type Config struct {
	Token   string
	BaseURL string

	Machines     int
	MaxMachines  bool
	FleetName    string
	UseStaticIPs bool

	Watch                 bool
	IncludePrimitiveNodes bool
	Ci                    bool

	NewWorkflowName string
	CreateMissing   bool

	OutputDirectory  string
	NodesToDownload  []string
	DownloadAllNodes bool

	RawInputs []string
	Inputs    workflowbuilder.Inputs

	ConfigFile string

	WorkflowSpec config.WorkflowRunSpec
}

var cfg = &Config{}

func init() {
	ExecuteCmd.Flags().IntVar(&cfg.Machines, "machines", DefaultMachines, "The number of machines to use for the workflow execution")
	ExecuteCmd.Flags().BoolVar(&cfg.MaxMachines, "max", false, "Use maximum number of machines for workflow execution")
	ExecuteCmd.Flags().StringVar(&cfg.FleetName, "fleet", DefaultFleetName, "The name of the fleet to use to execute the workflow")

	if useStaticIPs, exists := os.LookupEnv("TRICKEST_USE_STATIC_IPS"); exists {
		cfg.UseStaticIPs = useStaticIPs == "true"
	}
	ExecuteCmd.Flags().BoolVar(&cfg.UseStaticIPs, "use-static-ips", cfg.UseStaticIPs, "Use static IP addresses for the execution")

	ExecuteCmd.Flags().StringSliceVar(&cfg.RawInputs, "input", []string{}, "Input to pass to the workflow in the format key=value (can be used multiple times)")

	ExecuteCmd.Flags().BoolVar(&cfg.Watch, "watch", false, "Watch the execution running")
	ExecuteCmd.Flags().BoolVar(&cfg.IncludePrimitiveNodes, "include-primitive-nodes", false, "Include primitive nodes in the workflow tree")
	ExecuteCmd.Flags().BoolVar(&cfg.Ci, "ci", false, "Run in CI mode (in-progreess executions will be stopped when the CLI is forcefully stopped - if not set, you will be asked for confirmation)")

	ExecuteCmd.Flags().StringVar(&cfg.NewWorkflowName, "set-name", "", "Set workflow name if it's imported from the library")
	ExecuteCmd.Flags().BoolVar(&cfg.CreateMissing, "create-missing", false, "Create space and project if they don't exist")

	ExecuteCmd.Flags().StringVar(&cfg.OutputDirectory, "output-dir", "", "Path to directory which should be used to store outputs")
	ExecuteCmd.Flags().StringSliceVar(&cfg.NodesToDownload, "output", []string{}, "Output to download when the execution is finished (can be used multiple times)")
	ExecuteCmd.Flags().BoolVar(&cfg.DownloadAllNodes, "output-all", false, "Download all outputs when the execution is finished")

	ExecuteCmd.Flags().StringVar(&cfg.ConfigFile, "config", "", "YAML file for run configuration")
}

// ExecuteCmd represents the execute command
var ExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute a workflow",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cfg.Token = util.GetToken()
		cfg.BaseURL = util.Cfg.BaseUrl
		cfg.WorkflowSpec = config.WorkflowRunSpec{
			SpaceName:    util.SpaceName,
			ProjectName:  util.ProjectName,
			WorkflowName: util.WorkflowName,
			URL:          util.URL,
		}
		if err := run(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func run(cfg *Config) error {
	if cfg.ConfigFile != "" {
		runConfig, err := config.ParseConfigFile(cfg.ConfigFile)
		if err != nil {
			return fmt.Errorf("failed to parse run config: %w", err)
		}
		if cfg.Machines == DefaultMachines && runConfig.Machines > 0 {
			cfg.Machines = runConfig.Machines
		}
		if cfg.FleetName == DefaultFleetName && runConfig.Fleet != "" {
			cfg.FleetName = runConfig.Fleet
		}
		if !cfg.UseStaticIPs {
			cfg.UseStaticIPs = runConfig.UseStaticIPs
		}
		cfg.Inputs.NodeInputs = append(cfg.Inputs.NodeInputs, runConfig.NodeInputs...)
		cfg.Inputs.PrimitiveNodeInputs = append(cfg.Inputs.PrimitiveNodeInputs, runConfig.PrimitiveNodeInputs...)
		cfg.NodesToDownload = append(cfg.NodesToDownload, runConfig.Outputs...)
	}

	for _, rawInput := range cfg.RawInputs {
		inputMap := make(map[string]any)
		parts := strings.SplitN(rawInput, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid input format %q, expected format: key=value", rawInput)
		}
		key := parts[0]
		value := parts[1]
		inputMap[key] = value

		nodeInput, primitiveNodeInput, err := config.ParseInputs(inputMap)
		if err != nil {
			return fmt.Errorf("failed to parse input %q: %w", rawInput, err)
		}

		cfg.Inputs.NodeInputs = append(cfg.Inputs.NodeInputs, nodeInput...)
		cfg.Inputs.PrimitiveNodeInputs = append(cfg.Inputs.PrimitiveNodeInputs, primitiveNodeInput...)
	}

	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	fleet, err := client.GetFleetByName(ctx, cfg.FleetName)
	if err != nil {
		return fmt.Errorf("failed to get fleet: %w", err)
	}

	if cfg.UseStaticIPs {
		ipAddresses, err := client.GetVaultIPAddresses(ctx)
		if err != nil {
			return fmt.Errorf("failed to get static IP addresses: %w", err)
		}

		if len(ipAddresses) == 0 {
			return fmt.Errorf("static IP addresses are not enabled for your account - please contact support to enable this feature, or run without static IPs by setting TRICKEST_USE_STATIC_IPS=false or removing the --use-static-ips flag")
		}
	}

	if cfg.WorkflowSpec.SpaceName != "" {
		if err := cfg.WorkflowSpec.ResolveSpaceAndProject(ctx, client); err != nil {
			if cfg.CreateMissing {
				if err := cfg.WorkflowSpec.CreateMissing(ctx, client); err != nil {
					return fmt.Errorf("failed to create missing space/project: %w", err)
				}
			} else {
				return fmt.Errorf("failed to get space/project: %w", err)
			}
		}
	}

	workflow, err := cfg.WorkflowSpec.GetWorkflow(ctx, client)
	if err != nil {
		if cfg.WorkflowSpec.Space == nil {
			return fmt.Errorf("failed to get workflow: %w", err)
		}

		var projectID uuid.UUID
		if cfg.WorkflowSpec.Project != nil {
			projectID = *cfg.WorkflowSpec.Project.ID
		}

		workflow, err = getWorkflowFromLibrary(ctx, client, cfg.WorkflowSpec.WorkflowName, *cfg.WorkflowSpec.Space.ID, projectID, cfg.NewWorkflowName)
		if err != nil {
			return fmt.Errorf("failed to get workflow %q from a space or library: %w", cfg.WorkflowSpec.WorkflowName, err)
		}
	}

	workflowVersion, err := client.GetLatestWorkflowVersion(ctx, workflow.ID)
	if err != nil {
		return fmt.Errorf("failed to get workflow version: %w", err)
	}

	workflowMaxMachines, err := client.GetWorkflowVersionMaxMachines(ctx, workflowVersion.ID, fleet.ID)
	if err != nil {
		return fmt.Errorf("failed to get workflow version max machine count: %w", err)
	}
	if cfg.Machines > workflowMaxMachines {
		return fmt.Errorf("the number of machines specified (%d) is greater than the maximum possible for this workflow (%d)", cfg.Machines, workflowMaxMachines)
	}

	if cfg.MaxMachines {
		cfg.Machines = workflowMaxMachines
	}

	lookupTable := workflowbuilder.BuildNodeLookupTable(workflowVersion)
	if err := lookupTable.ResolveInputs(&cfg.Inputs); err != nil {
		return fmt.Errorf("failed to pass inputs to workflow: %w", err)
	}

	for _, input := range cfg.Inputs.NodeInputs {
		for paramName, paramValues := range input.ParamValues {
			inputType, err := lookupTable.GetNodeInputType(input.NodeID, paramName)
			if err != nil {
				return fmt.Errorf("failed to get node input type: %w", err)
			}
			for i, paramValue := range paramValues {
				if inputType == "FILE" && isLocalFile(paramValue.(string)) {
					file, err := client.UploadFile(ctx, paramValue.(string), true)
					if err != nil {
						return fmt.Errorf("failed to upload file: %w", err)
					}
					paramValues[i] = fmt.Sprintf("trickest://file/%s", file.Name)
				}
			}
		}
		err = input.ApplyToWorkflowVersion(workflowVersion)
		if err != nil {
			return fmt.Errorf("failed to apply node input: %w", err)
		}
	}

	for _, input := range cfg.Inputs.PrimitiveNodeInputs {
		inputType, err := lookupTable.GetPrimitiveNodeInputType(input.PrimitiveNodeID)
		if err != nil {
			return fmt.Errorf("failed to get primitive node input type: %w", err)
		}
		if inputType == "FILE" && isLocalFile(input.Value.(string)) {
			file, err := client.UploadFile(ctx, input.Value.(string), true)
			if err != nil {
				return fmt.Errorf("failed to upload file: %w", err)
			}
			input.Value = fmt.Sprintf("trickest://file/%s", file.Name)
		}
		err = input.ApplyToWorkflowVersion(workflowVersion)
		if err != nil {
			return fmt.Errorf("failed to apply primitive node input: %w", err)
		}
	}

	workflowVersion, err = client.CreateWorkflowVersion(ctx, workflowVersion)
	if err != nil {
		return fmt.Errorf("failed to create workflow version: %w", err)
	}

	run, err := client.CreateRun(ctx, workflowVersion.ID, cfg.Machines, *fleet, cfg.UseStaticIPs)
	if err != nil {
		return fmt.Errorf("failed to create run: %w", err)
	}
	// The extra space aligns the Run ID with other fields displayed by the run watcher
	// The run ID is printed here because it should be displayed whether the run is watched or not
	fmt.Printf("%-18s %v\n", "Run ID:", *run.ID)

	if cfg.DownloadAllNodes || len(cfg.NodesToDownload) > 0 {
		cfg.Watch = true
	}

	if cfg.Watch {
		watcher, err := display.NewRunWatcher(
			client,
			*run.ID,
			display.WithWorkflowVersion(workflowVersion),
			display.WithIncludePrimitiveNodes(cfg.IncludePrimitiveNodes),
			display.WithCI(cfg.Ci),
		)
		if err != nil {
			return fmt.Errorf("failed to create run watcher: %w", err)
		}
		err = watcher.Watch(ctx)
		if err != nil {
			return fmt.Errorf("failed to watch run: %w", err)
		}
	}

	if cfg.DownloadAllNodes || len(cfg.NodesToDownload) > 0 {
		run, err := client.GetRun(ctx, *run.ID) // Refresh the run to get the startedDate
		if err != nil {
			return fmt.Errorf("failed to get run: %w", err)
		}
		results, runDir, err := actions.DownloadRunOutput(client, run, cfg.NodesToDownload, []string{}, cfg.OutputDirectory)
		if err != nil {
			return fmt.Errorf("failed to download run outputs: %w", err)
		}
		actions.PrintDownloadResults(results, *run.ID, runDir)
	}

	return nil
}

func isLocalFile(value string) bool {
	return !(strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "trickest://file/") || strings.HasPrefix(value, "trickest://output/"))
}

func getWorkflowFromLibrary(ctx context.Context, client *trickest.Client, workflowName string, spaceID uuid.UUID, projectID uuid.UUID, newWorkflowName string) (*trickest.Workflow, error) {
	if workflowName == "" {
		return nil, fmt.Errorf("workflow name cannot be empty")
	}
	if spaceID == uuid.Nil {
		return nil, fmt.Errorf("space ID cannot be nil")
	}

	libraryWorkflow, err := client.GetLibraryWorkflowByName(ctx, workflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow %q from library: %w", workflowName, err)
	}

	workflow, err := client.CopyWorkflowFromLibrary(ctx, libraryWorkflow.ID, spaceID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to copy workflow %q to space %s: %w", workflowName, spaceID, err)
	}

	if newWorkflowName != "" {
		updatedWorkflow, err := client.RenameWorkflow(ctx, workflow.ID, newWorkflowName)
		if err != nil {
			return nil, fmt.Errorf("failed to rename copied workflow %q to %q: %w", workflow.Name, newWorkflowName, err)
		}
		workflow = *updatedWorkflow
	}

	return &workflow, nil
}
