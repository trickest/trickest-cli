package files

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/types"
	"github.com/xlab/treeprint"
)

var (
	searchQuery string
	jsonOutput  bool
)

// filesListCmd represents the filesGet command
var filesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List files in the Trickest file storage",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		files, err := getMetadata(searchQuery)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		} else {
			printFiles(files, jsonOutput)
		}
	},
}

func init() {
	FilesCmd.AddCommand(filesListCmd)

	filesListCmd.Flags().StringVar(&searchQuery, "query", "", "Filter listed files using the specified search query")
	filesListCmd.Flags().BoolVar(&jsonOutput, "json", false, "Display output in JSON format")
}

func printFiles(files []types.File, jsonOutput bool) {
	var output string

	if jsonOutput {
		data, err := json.Marshal(files)
		if err != nil {
			fmt.Println("Error marshalling response data")
			return
		}
		output = string(data)
	} else {
		tree := treeprint.New()
		tree.SetValue("Files")
		for _, file := range files {
			fileSubBranch := tree.AddBranch("\U0001f4c4 " + file.Name)                             //📄
			fileSubBranch.AddNode("\U0001f522 " + file.PrettySize)                                 //🔢
			fileSubBranch.AddNode("\U0001f4c5 " + file.ModifiedDate.Format("2006-01-02 15:04:05")) //📅
		}

		output = tree.String()
	}

	fmt.Println(output)
}
