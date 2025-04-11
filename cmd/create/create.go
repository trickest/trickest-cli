package create

import (
	"context"
	"fmt"
	"os"

	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
)

type Config struct {
	Token   string
	BaseURL string

	SpaceName          string
	ProjectName        string
	SpaceDescription   string
	ProjectDescription string
}

var cfg = &Config{}

func init() {
	CreateCmd.Flags().StringVar(&cfg.SpaceName, "space", "", "Name of the space to create")
	CreateCmd.Flags().StringVar(&cfg.SpaceDescription, "space-description", "", "Description for the space")
	CreateCmd.Flags().StringVar(&cfg.ProjectName, "project", "", "Name of the project to create")
	CreateCmd.Flags().StringVar(&cfg.ProjectDescription, "project-description", "", "Description for the project")
}

// CreateCmd represents the create command
var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a space or a project on the Trickest platform",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.SpaceName == "" && cfg.ProjectName == "" {
			fmt.Fprintf(os.Stderr, "Error: space or project name is required\n")
			os.Exit(1)
		}
		cfg.Token = util.GetToken()
		cfg.BaseURL = util.Cfg.BaseUrl
		if err := run(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
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

	var space *trickest.Space
	spaceCreated := false
	if cfg.SpaceName != "" {
		space, err = client.GetSpaceByName(ctx, cfg.SpaceName)
		if err != nil {
			space, err = client.CreateSpace(ctx, cfg.SpaceName, cfg.SpaceDescription)
			if err != nil {
				return fmt.Errorf("failed to create space: %w", err)
			}
			spaceCreated = true
		}
	}

	projectCreated := false
	if cfg.ProjectName != "" {
		_, err := space.GetProjectByName(cfg.ProjectName)
		if err != nil {
			_, err = client.CreateProject(ctx, cfg.ProjectName, cfg.ProjectDescription, *space.ID)
			if err != nil {
				return fmt.Errorf("failed to create project: %w", err)
			}
			projectCreated = true
		}
	}

	if cfg.ProjectName != "" {
		if !projectCreated {
			return fmt.Errorf("project %q already exists", cfg.ProjectName)
		}
	} else if !spaceCreated {
		return fmt.Errorf("space %q already exists", cfg.SpaceName)
	}

	return nil
}
