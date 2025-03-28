package help

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/trickest/trickest-cli/pkg/config"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/pkg/workflowbuilder"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
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

func generateHelpMarkdown(workflow *trickest.Workflow, labeledNodes []*trickest.PrimitiveNode, workflowSpec config.WorkflowRunSpec) string {
	var sb strings.Builder
	workflowURL := constructWorkflowURL(workflow)

	// Title
	sb.WriteString(fmt.Sprintf("# %s\n\n", workflow.Name))

	// Description if it exists
	if workflow.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", workflow.Description))
	}

	// Author info
	sb.WriteString(fmt.Sprintf("**Author:** %s\n\n", workflow.Author))

	// Inputs section
	if len(labeledNodes) > 0 {
		sb.WriteString("## Inputs\n\n")
		// Sort nodes by their position on the workflow canvas
		sort.Slice(labeledNodes, func(i, j int) bool {
			return labeledNodes[i].Coordinates.Y < labeledNodes[j].Coordinates.Y
		})
		sb.WriteString("These are the current workflow input values. When you execute the workflow, you only need to set the inputs you want to change.\n\n")
		for _, node := range labeledNodes {
			inputLine := fmt.Sprintf("- `%s` (%s)", node.Label, strings.ToLower(node.Type))
			if node.Value != "" {
				inputLine += fmt.Sprintf(" = %s", node.Value)
			}
			sb.WriteString(fmt.Sprintf("%s\n", inputLine))
		}
		sb.WriteString("\n")
	}

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
	for _, node := range labeledNodes {
		nodeValue := fmt.Sprintf("<%s-value>", strings.ReplaceAll(node.Label, " ", "-"))
		if node.Value != "" {
			nodeValue = node.Value.(string)
		}
		exampleCommand += fmt.Sprintf(" --inputs \"%s=%s\"", node.Label, nodeValue)
	}
	sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", exampleCommand))

	// Long description (README content)
	if workflow.LongDescription != "" {
		sb.WriteString("## Description\n\n")
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

	labeledNodes, err := workflowbuilder.GetLabeledPrimitiveNodes(workflowVersion)
	if err != nil {
		return fmt.Errorf("failed to get labeled primitive nodes: %w", err)
	}

	helpMarkdown := generateHelpMarkdown(workflow, labeledNodes, cfg.WorkflowSpec)
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
