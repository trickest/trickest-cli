package scripts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/go-yaml/yaml"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"
)

var ScriptsCmd = &cobra.Command{
	Use:   "scripts",
	Short: "Manage private scripts",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	ScriptsCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = ScriptsCmd.Flags().MarkHidden("workflow")
		_ = ScriptsCmd.Flags().MarkHidden("project")
		_ = ScriptsCmd.Flags().MarkHidden("space")
		_ = ScriptsCmd.Flags().MarkHidden("url")

		command.Root().HelpFunc()(command, strings)
	})
}

func ListPrivateScripts(name string) ([]types.Script, error) {
	endpoint := fmt.Sprintf("script/?vault=%s", util.GetVault())
	if name != "" {
		endpoint += "&search=" + name
	} else {
		endpoint += "&page_size=100"
	}

	resp := request.Trickest.Get().Do(endpoint)
	if resp == nil || resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var scripts types.Scripts
	err := json.Unmarshal(resp.Body(), &scripts)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse API response: %s", err)
	}

	return scripts.Results, nil
}

func getScriptIDByName(name string) (uuid.UUID, error) {
	scripts, err := ListPrivateScripts(name)
	if err != nil {
		return uuid.Nil, fmt.Errorf("couldn't search for %s: %w", name, err)
	}

	for _, script := range scripts {
		if script.Name == name {
			return script.ID, nil
		}
	}

	return uuid.Nil, fmt.Errorf("couldn't find script '%s'", name)
}

func createScriptImportRequestFromYAML(fileName string) (types.ScriptImportRequest, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		err = fmt.Errorf("couldn't read %s: %w", fileName, err)
		return types.ScriptImportRequest{}, err
	}

	var scriptImportRequest types.ScriptImportRequest
	err = yaml.Unmarshal(data, &scriptImportRequest)
	if err != nil {
		err = fmt.Errorf("couldn't parse %s: %w", fileName, err)
		return types.ScriptImportRequest{}, err
	}

	scriptImportRequest.VaultInfo = util.GetVault()

	return scriptImportRequest, nil
}

func importScript(fileName string, isUpdate bool) (string, uuid.UUID, error) {
	scriptImportRequest, err := createScriptImportRequestFromYAML(fileName)
	if err != nil {
		return "", uuid.Nil, err
	}

	scriptJSON, err := json.Marshal(scriptImportRequest)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("couldn't encode %s: %w", fileName, err)
	}

	var resp *request.Response
	if isUpdate {
		scriptName := scriptImportRequest.Name
		scriptID, err := getScriptIDByName(scriptName)
		if err != nil {
			return "", uuid.Nil, fmt.Errorf("couldn't import '%s': %w", scriptName, err)
		}
		resp = request.Trickest.Patch().Body(scriptJSON).DoF("script/%s/", scriptID.String())
	} else {
		resp = request.Trickest.Post().Body(scriptJSON).Do("script/")
	}

	if resp == nil {
		return "", uuid.Nil, fmt.Errorf("couldn't import %s", fileName)
	}

	if resp.Status() != http.StatusCreated && resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var importedScript types.ScriptImportRequest
	err = json.Unmarshal(resp.Body(), &importedScript)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("couldn't import %s: %w", fileName, err)
	}

	return importedScript.Name, *importedScript.ID, nil
}
