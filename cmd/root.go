package cmd

import (
	"log"

	"github.com/trickest/trickest-cli/cmd/create"
	"github.com/trickest/trickest-cli/cmd/delete"
	"github.com/trickest/trickest-cli/cmd/execute"
	"github.com/trickest/trickest-cli/cmd/get"
	"github.com/trickest/trickest-cli/cmd/list"
	"github.com/trickest/trickest-cli/cmd/output"
	"github.com/trickest/trickest-cli/cmd/store"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "trickest",
	Short: "Trickest client for platform access from your local machine",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	log.SetFlags(0)
	cobra.CheckErr(RootCmd.Execute())
}

func init() {
	RootCmd.PersistentFlags().StringVar(&util.Cfg.User.Token, "token", "", "Trickest authentication token")
	RootCmd.PersistentFlags().StringVar(&util.Cfg.User.TokenFilePath, "token-file", "", "Trickest authentication token file")
	RootCmd.PersistentFlags().StringVar(&util.SpaceName, "space", "", "Space name")
	RootCmd.PersistentFlags().StringVar(&util.ProjectName, "project", "", "Project name")
	RootCmd.PersistentFlags().StringVar(&util.WorkflowName, "workflow", "", "Workflow name")
	RootCmd.PersistentFlags().StringVar(&util.Cfg.Dependency, "node-dependency", "", "This flag doesn't affect the execution logic of the CLI in any way and is intended for controlling node execution order on the Trickest platform only.")

	cobra.OnInitialize(util.CreateRequest, initVaultID)

	RootCmd.AddCommand(list.ListCmd)
	RootCmd.AddCommand(store.StoreCmd)
	RootCmd.AddCommand(create.CreateCmd)
	RootCmd.AddCommand(delete.DeleteCmd)
	RootCmd.AddCommand(output.OutputCmd)
	RootCmd.AddCommand(execute.ExecuteCmd)
	RootCmd.AddCommand(get.GetCmd)
	// RootCmd.AddCommand(export.ExportCmd)
}

func initVaultID() {
	util.GetVault()
}
