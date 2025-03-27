package scripts

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"
)

type DeleteConfig struct {
	Token   string
	BaseURL string

	ScriptID   string
	ScriptName string
}

var deleteCfg = &DeleteConfig{}

func init() {
	ScriptsCmd.AddCommand(scriptsDeleteCmd)

	scriptsDeleteCmd.Flags().StringVar(&deleteCfg.ScriptID, "id", "", "ID of the script to delete")
	scriptsDeleteCmd.Flags().StringVar(&deleteCfg.ScriptName, "name", "", "Name of the script to delete")
}

var scriptsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a private script",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if deleteCfg.ScriptName == "" && deleteCfg.ScriptID == "" {
			fmt.Fprintf(os.Stderr, "Error: script ID or name is required\n")
			os.Exit(1)
		}

		if deleteCfg.ScriptID != "" && deleteCfg.ScriptName != "" {
			fmt.Fprintf(os.Stderr, "Error: script ID and name cannot both be provided\n")
			os.Exit(1)
		}

		deleteCfg.Token = util.GetToken()
		deleteCfg.BaseURL = util.Cfg.BaseUrl
		if err := runDelete(deleteCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runDelete(cfg *DeleteConfig) error {
	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	var scriptID uuid.UUID

	if deleteCfg.ScriptID != "" {
		scriptID, err = uuid.Parse(deleteCfg.ScriptID)
		if err != nil {
			return fmt.Errorf("failed to parse script ID: %w", err)
		}
	} else {
		script, err := client.GetPrivateScriptByName(ctx, deleteCfg.ScriptName)
		if err != nil {
			return fmt.Errorf("failed to find script: %w", err)
		}
		scriptID = *script.ID
	}

	err = client.DeletePrivateScript(ctx, scriptID)
	if err != nil {
		return fmt.Errorf("failed to delete script: %w", err)
	}

	if deleteCfg.ScriptName != "" {
		fmt.Printf("Succesfuly deleted %q\n", deleteCfg.ScriptName)
	} else {
		fmt.Printf("Succesfuly deleted script (ID: %s)\n", scriptID)
	}
	return nil
}
