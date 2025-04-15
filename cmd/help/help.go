package help

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/trickest/trickest-cli/cmd/execute"
	"github.com/trickest/trickest-cli/pkg/config"
	display "github.com/trickest/trickest-cli/pkg/display/run"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/pkg/workflowbuilder"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
)

const (
	defaultMachineCount = 10
)

type Config struct {
	Token   string
	BaseURL string

	WorkflowSpec config.WorkflowRunSpec
}

var cfg = &Config{}

// HelpCmd represents the help command
var HelpCmd = &cobra.Command{
	Use:   "help",
	Short: "Get help for a workflow",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if util.URL == "" && util.WorkflowName == "" {
			fmt.Println("Error: the workflow name or URL must be provided")
			os.Exit(1)
		}
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

func generateHelpMarkdown(workflow *trickest.Workflow, labeledPrimitiveNodes []*trickest.PrimitiveNode, labeledNodes []*trickest.Node, workflowSpec config.WorkflowRunSpec, runs []trickest.Run, maxMachines int) string {
	workflowURL := constructWorkflowURL(workflow)

	// Sort input nodes by their position on the workflow canvas on the Y axis (top to bottom)
	sort.Slice(labeledPrimitiveNodes, func(i, j int) bool {
		return labeledPrimitiveNodes[i].Coordinates.Y < labeledPrimitiveNodes[j].Coordinates.Y
	})

	// Sort output nodes by their position on the workflow canvas on the X axis (right to left)
	sort.Slice(labeledNodes, func(i, j int) bool {
		return labeledNodes[i].Meta.Coordinates.X > labeledNodes[j].Meta.Coordinates.X
	})

	var sb strings.Builder

	// Title
	sb.WriteString(fmt.Sprintf("# %s\n\n", workflow.Name))

	// Description if it exists
	if workflow.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", workflow.Description))
	}

	runStats := []struct {
		date     time.Time
		machines int
		duration time.Duration
		url      string
	}{}
	for _, run := range runs {
		machines := run.Machines.Default
		if machines == nil {
			machines = run.Machines.SelfHosted
		}
		date := *run.StartedDate
		duration := run.CompletedDate.Sub(date)
		runURL := fmt.Sprintf("%s?run=%s", workflowURL, run.ID)
		runStats = append(runStats, struct {
			date     time.Time
			machines int
			duration time.Duration
			url      string
		}{date, *machines, duration, runURL})
	}

	machineCount := defaultMachineCount
	if maxMachines > 0 && maxMachines < defaultMachineCount {
		machineCount = maxMachines
	} else if len(runStats) > 0 {
		highestMachineCount := 0
		for _, runStat := range runStats {
			if runStat.machines > highestMachineCount {
				highestMachineCount = runStat.machines
			}
		}
		machineCount = highestMachineCount
	}

	// Author info
	sb.WriteString(fmt.Sprintf("**Author:** %s\n\n", workflow.Author))

	// Example command
	sb.WriteString("## Example Command\n\n")
	exampleCommand := fmt.Sprintf("%s execute", os.Args[0])
	workflowRef := ""
	// Use the same reference format the user used to run this command
	if workflowSpec.URL != "" {
		workflowRef = fmt.Sprintf("--url \"%s\"", workflowURL)
	} else {
		workflowRef = fmt.Sprintf("--space \"%s\"", workflowSpec.SpaceName)
		if workflowSpec.ProjectName != "" {
			workflowRef += fmt.Sprintf(" --project \"%s\"", workflowSpec.ProjectName)
		}
		workflowRef += fmt.Sprintf(" --workflow \"%s\"", workflowSpec.WorkflowName)
	}
	exampleCommand += fmt.Sprintf(" %s", workflowRef)
	// Add inputs with example values
	for _, node := range labeledPrimitiveNodes {
		nodeValue := getPrimitiveNodeValue(node)
		if nodeValue == "" {
			nodeValue = fmt.Sprintf("<%s-value>", strings.ReplaceAll(node.Label, " ", "-"))
		}
		exampleCommand += fmt.Sprintf(" --input \"%s=%s\"", node.Label, nodeValue)
	}
	// Add the first output only to avoid cluttering the command too much
	if len(labeledNodes) > 0 {
		exampleCommand += fmt.Sprintf(" --output \"%s\"", labeledNodes[0].Meta.Label)
	}

	if machineCount > 1 {
		exampleCommand += fmt.Sprintf(" --machines %d", machineCount)
	}
	sb.WriteString(fmt.Sprintf("```bash\n%s\n```\n\n", exampleCommand))

	// Inputs section
	if len(labeledPrimitiveNodes) > 0 {
		sb.WriteString("## Inputs\n\n")
		for _, node := range labeledPrimitiveNodes {
			inputLine := fmt.Sprintf("- `%s` (%s)", node.Label, strings.ToLower(node.Type))
			nodeValue := getPrimitiveNodeValue(node)
			if nodeValue != "" {
				inputLine += fmt.Sprintf(" = %s", nodeValue)
			}
			sb.WriteString(fmt.Sprintf("%s\n", inputLine))
		}
		sb.WriteString("\n\n")
		sb.WriteString("Use the `--input` flag to set the inputs you want to change.\n\n")
	}

	// Outputs section
	if len(labeledNodes) > 0 {
		sb.WriteString("## Outputs\n\n")
		for _, node := range labeledNodes {
			sb.WriteString(fmt.Sprintf("- `%s`\n", node.Meta.Label))
		}
		sb.WriteString("\n\n")
		sb.WriteString("Use the `--output` flag to specify the outputs you want to get.\n\n")
	}

	// Past runs section
	if len(runStats) > 0 {
		sb.WriteString("## Past Runs\n\n")
		sb.WriteString("| Started at | Machines | Duration | URL |\n")
		sb.WriteString("|------------|----------|----------|-----|\n")
		for _, runStat := range runStats {
			machines := runStat.machines
			date := runStat.date.Format("2006-01-02 15:04")
			duration := runStat.duration
			durationStr := display.FormatDuration(duration)
			runURL := runStat.url
			sb.WriteString(fmt.Sprintf("| %s | %d | %s | [View](%s) |\n", date, machines, durationStr, runURL))
		}
		sb.WriteString("\n")
		sb.WriteString("Use the `--machines` flag to set the number of machines to run the workflow on.\n\n")
	}

	// Long description (README content)
	if workflow.LongDescription != "" {
		sb.WriteString("## Author's Notes\n\n")
		sb.WriteString(workflow.LongDescription)
		sb.WriteString("\n\n")
	}

	// Links to the workflow and execute docs
	sb.WriteString("## Links\n\n")
	sb.WriteString(fmt.Sprintf("- [View on Trickest](%s)\n", workflowURL))
	sb.WriteString("- [Learn more about executing workflows](https://github.com/trickest/trickest-cli#execute)")

	return sb.String()
}

func run(cfg *Config) error {
	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	workflow, err := cfg.WorkflowSpec.GetWorkflow(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	workflowVersion, err := client.GetLatestWorkflowVersion(ctx, workflow.ID)
	if err != nil {
		return fmt.Errorf("failed to get workflow version: %w", err)
	}

	versionMaxMachines := 0
	fleet, err := client.GetFleetByName(ctx, execute.DefaultFleetName)
	if err == nil {
		maxMachines, err := client.GetWorkflowVersionMaxMachines(ctx, workflowVersion.ID, fleet.ID)
		if err == nil {
			if maxMachines.Default != nil {
				versionMaxMachines = *maxMachines.Default
			}
			if maxMachines.SelfHosted != nil {
				versionMaxMachines = *maxMachines.SelfHosted
			}
		}
	}

	labeledPrimitiveNodes, err := workflowbuilder.GetLabeledPrimitiveNodes(workflowVersion)
	if err != nil {
		return fmt.Errorf("failed to get labeled primitive nodes: %w", err)
	}

	labeledNodes, err := workflowbuilder.GetLabeledNodes(workflowVersion)
	if err != nil {
		return fmt.Errorf("failed to get labeled nodes: %w", err)
	}

	runs, err := client.GetRuns(ctx, workflow.ID, "COMPLETED", 5)
	if err != nil {
		return fmt.Errorf("failed to get runs: %w", err)
	}

	helpMarkdown := generateHelpMarkdown(workflow, labeledPrimitiveNodes, labeledNodes, cfg.WorkflowSpec, runs, versionMaxMachines)
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(-1),
	)
	out, err := r.Render(helpMarkdown)
	if err != nil {
		return fmt.Errorf("failed to render help output: %w", err)
	}
	fmt.Println(out)

	return nil
}

func constructWorkflowURL(workflow *trickest.Workflow) string {
	return fmt.Sprintf("https://trickest.io/editor/%s", workflow.ID)
}

func getPrimitiveNodeValue(node *trickest.PrimitiveNode) string {
	if node.Type == "BOOLEAN" {
		return strconv.FormatBool(node.Value.(bool))
	}
	return fmt.Sprintf("%v", node.Value)
}
