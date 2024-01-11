package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"
)

var toolOutputTypes = map[string]string{
	"file":   "2",
	"folder": "3",
}

var ToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Manage private tools",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	ToolsCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = ToolsCmd.Flags().MarkHidden("workflow")
		_ = ToolsCmd.Flags().MarkHidden("project")
		_ = ToolsCmd.Flags().MarkHidden("space")
		_ = ToolsCmd.Flags().MarkHidden("url")

		command.Root().HelpFunc()(command, strings)
	})
}

func ListPrivateTools(name string) ([]types.Tool, error) {
	endpoint := "library/tool/?public=False"
	endpoint += fmt.Sprintf("&vault=%s", util.GetVault())
	if name != "" {
		endpoint += "&search=" + name
	} else {
		endpoint += "&page_size=100"
	}

	resp := request.Trickest.Get().Do(endpoint)
	if resp == nil || resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var tools types.Tools
	err := json.Unmarshal(resp.Body(), &tools)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse API response: %s", err)
	}

	return tools.Results, nil
}

func getToolIDByName(name string) (uuid.UUID, error) {
	tools, err := ListPrivateTools(name)
	if err != nil {
		return uuid.Nil, fmt.Errorf("couldn't search for %s: %w", name, err)
	}

	if len(tools) == 0 {
		return uuid.Nil, fmt.Errorf("couldn't find tool '%s'", name)
	}

	if len(tools) > 1 {
		return uuid.Nil, fmt.Errorf("found more than one match for '%s'", name)
	}

	return tools[0].ID, nil
}

func createToolImportRequestFromYAML(fileName string) (types.ToolImportRequest, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		err = fmt.Errorf("couldn't read %s: %w", fileName, err)
		return types.ToolImportRequest{}, err
	}

	var toolImportRequest types.ToolImportRequest
	err = yaml.Unmarshal(data, &toolImportRequest)
	if err != nil {
		err = fmt.Errorf("couldn't parse %s: %w", fileName, err)
		return types.ToolImportRequest{}, err
	}

	categoryID, err := util.GetCategoryIDByName(toolImportRequest.Category)
	if err != nil {
		err = fmt.Errorf("couldn't use the category '%s': %w", toolImportRequest.Category, err)
		return types.ToolImportRequest{}, err
	}

	toolImportRequest.CategoryID = categoryID
	toolImportRequest.VaultInfo = util.GetVault()
	toolImportRequest.OutputType = toolOutputTypes[toolImportRequest.OutputType]
	for name := range toolImportRequest.Inputs {
		if input, ok := toolImportRequest.Inputs[name]; ok {
			input.Type = strings.ToUpper(toolImportRequest.Inputs[name].Type)
			toolImportRequest.Inputs[name] = input
		}
	}

	return toolImportRequest, nil
}

func importTool(fileName string, isUpdate bool) (string, uuid.UUID, error) {
	toolImportRequest, err := createToolImportRequestFromYAML(fileName)
	if err != nil {
		return "", uuid.Nil, err
	}

	toolJSON, err := json.Marshal(toolImportRequest)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("couldn't encode %s: %w", fileName, err)
	}

	var resp *request.Response
	if isUpdate {
		toolName := toolImportRequest.Name
		toolID, err := getToolIDByName(toolName)
		if err != nil {
			return "", uuid.Nil, fmt.Errorf("couldn't import '%s': %w", toolName, err)
		}
		resp = request.Trickest.Patch().Body(toolJSON).DoF("library/tool/%s/", toolID.String())
	} else {
		resp = request.Trickest.Post().Body(toolJSON).Do("library/tool/")
	}

	if resp == nil {
		return "", uuid.Nil, fmt.Errorf("couldn't import %s", fileName)
	}

	if resp.Status() != http.StatusCreated && resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var importedTool types.Tool
	err = json.Unmarshal(resp.Body(), &importedTool)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("couldn't import %s: %w", fileName, err)
	}

	return importedTool.Name, importedTool.ID, nil
}
