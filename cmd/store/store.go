package store

import (
	"github.com/spf13/cobra"
)

// StoreCmd represents the store command
var StoreCmd = &cobra.Command{
	Use:   "store",
	Short: "Browse workflows in the Trickest store",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {

}
