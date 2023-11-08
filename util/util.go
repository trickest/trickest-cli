package util

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/types"

	"github.com/google/uuid"

	"github.com/hako/durafmt"
)

type UnexpectedResponse map[string]interface{}

const (
	BaseURL = "https://hive-api.trickest.io/"
)

var (
	Cfg = types.Config{
		BaseUrl: BaseURL,
	}
	SpaceName    string
	ProjectName  string
	WorkflowName string
	URL          string
)

func CreateRequest() {
	request.Trickest = request.New().Endpoint(Cfg.BaseUrl).Version("v1").Header("Authorization", "Token "+GetToken())
}

func FormatPath() string {
	path := strings.Trim(SpaceName, "/")
	if ProjectName != "" {
		path += "/" + strings.Trim(ProjectName, "/")
	}
	if WorkflowName != "" {
		path += "/" + strings.Trim(WorkflowName, "/")
	}
	return path
}

func GetToken() string {
	if Cfg.User.Token != "" {
		return Cfg.User.Token
	}

	if Cfg.User.TokenFilePath != "" {
		token, err := os.ReadFile(Cfg.User.TokenFilePath)
		if err != nil {
			log.Fatal("Couldn't read the token file: ", err)
		}
		Cfg.User.Token = string(token)
		return Cfg.User.Token
	}

	if tokenEnv, tokenSet := os.LookupEnv("TRICKEST_TOKEN"); tokenSet {
		Cfg.User.Token = tokenEnv
		return tokenEnv
	}

	log.Fatal("Trickest authentication token not set! Use --token, --token-file, or TRICKEST_TOKEN environment variable.")
	return ""
}

func GetVault() uuid.UUID {
	if Cfg.User.VaultId == uuid.Nil {
		user := GetMe()
		if user != nil {
			Cfg.User.VaultId = user.Profile.VaultInfo.ID
		} else {
			fmt.Println("Couldn't get default vault ID! Check your Trickest token.")
			os.Exit(0)
		}
	}
	return Cfg.User.VaultId
}

func GetMe() *types.User {
	resp := request.Trickest.Get().DoF("users/me/")
	if resp == nil || resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var user types.User
	err := json.Unmarshal(resp.Body(), &user)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal user info.")
		os.Exit(0)
	}

	return &user
}

func GetFleetInfo() *types.Fleet {
	resp := request.Trickest.Get().DoF("fleet/?vault=%s", GetVault())
	if resp == nil || resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var fleets types.Fleets
	err := json.Unmarshal(resp.Body(), &fleets)
	if err != nil {
		fmt.Println("Error unmarshalling fleet response!")
		return nil
	}

	if len(fleets.Results) == 0 {
		fmt.Println("Error: Couldn't find any active fleets")
	}

	return &fleets.Results[0]
}

func GetSpaces(name string) []types.Space {
	urlReq := "spaces/?vault=" + GetVault().String()
	urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)

	if name != "" {
		urlReq += "&name=" + url.QueryEscape(name)
	}

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get spaces!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var spaces types.Spaces
	err := json.Unmarshal(resp.Body(), &spaces)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal spaces response!")
		os.Exit(0)
	}

	return spaces.Results
}

func GetSpaceByName(name string) *types.SpaceDetailed {
	spaces := GetSpaces(name)
	if len(spaces) == 0 {
		return nil
	}

	return getSpaceByID(spaces[0].ID)
}

func getSpaceByID(id uuid.UUID) *types.SpaceDetailed {
	resp := request.Trickest.Get().DoF("spaces/%s/", id.String())
	if resp == nil {
		fmt.Println("Error: Couldn't get space by ID!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var space types.SpaceDetailed
	err := json.Unmarshal(resp.Body(), &space)
	if err != nil {
		fmt.Println("Error unmarshalling space response!")
		os.Exit(0)
	}

	return &space
}

func getProjectByID(id uuid.UUID) *types.Project {
	resp := request.Trickest.Get().DoF("projects/%s/", id.String())
	if resp == nil {
		fmt.Println("Error: Couldn't get project by ID!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var project types.Project
	err := json.Unmarshal(resp.Body(), &project)
	if err != nil {
		fmt.Println("Error unmarshalling project response!")
		os.Exit(0)
	}

	return &project
}

func GetWorkflows(projectID, spaceID uuid.UUID, search string, library bool) []types.Workflow {
	urlReq := "workflow/"
	if library {
		urlReq = "library/" + urlReq
	}
	urlReq += "?page_size=" + strconv.Itoa(math.MaxInt)
	if !library {
		urlReq += "&vault=" + GetVault().String()
	}

	if search != "" {
		urlReq += "&search=" + url.QueryEscape(search)
	}

	if projectID != uuid.Nil {
		urlReq += "&project=" + projectID.String()
	} else if spaceID != uuid.Nil {
		urlReq += "&space=" + spaceID.String()
	}

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get workflows!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var workflows types.Workflows
	err := json.Unmarshal(resp.Body(), &workflows)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal workflows response!")
		os.Exit(0)
	}

	return workflows.Results
}

func GetWorkflowByID(id uuid.UUID) *types.Workflow {
	resp := request.Trickest.Get().DoF("workflow/%s/", id.String())
	if resp == nil {
		fmt.Println("Error: Couldn't get workflow by ID!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var workflow types.Workflow
	err := json.Unmarshal(resp.Body(), &workflow)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal workflow response!")
		os.Exit(0)
	}

	return &workflow
}

func ResolveObjectPath(path string, silent bool, isProject bool) (*types.SpaceDetailed, *types.Project, *types.Workflow, bool) {
	pathSplit := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathSplit) > 3 {
		if !silent {
			fmt.Println("Invalid object path!")
		}
		return nil, nil, nil, false
	}
	space := GetSpaceByName(pathSplit[0])
	if space == nil {
		if !silent {
			fmt.Println("Couldn't find space named " + pathSplit[0] + "!")
		}
		return nil, nil, nil, false
	}

	if len(pathSplit) == 1 {
		return space, nil, nil, true
	}

	// Space and workflow with no project
	var projectName string
	var workflowName string
	if len(pathSplit) == 2 {
		if isProject {
			projectName = pathSplit[1]
			workflowName = ""
		} else {
			projectName = ""
			workflowName = pathSplit[1]
		}
	} else {
		projectName = pathSplit[1]
		workflowName = pathSplit[2]
	}

	var project *types.Project
	if space.Projects != nil && len(space.Projects) > 0 {
		for _, proj := range space.Projects {
			if proj.Name == projectName {
				project = &proj
				project.Workflows = GetWorkflows(project.ID, uuid.Nil, "", false)
				break
			}
		}
	}

	var workflow *types.Workflow
	if space.Workflows != nil && len(space.Workflows) > 0 {
		for _, wf := range space.Workflows {
			if wf.Name == workflowName {
				workflow = &wf
				break
			}
		}
	}

	if len(pathSplit) == 2 {
		if project != nil || workflow != nil {
			return space, project, workflow, true
		}
		if workflow != nil {
			return space, nil, workflow, true
		}
		if !silent {
			fmt.Println("Couldn't find project or workflow named " + pathSplit[1] + " inside " +
				pathSplit[0] + " space!")
		}
		return space, nil, nil, false
	}

	if project != nil && project.Workflows != nil && len(project.Workflows) > 0 {
		for _, wf := range project.Workflows {
			if wf.Name == pathSplit[2] {
				fullWorkflow := GetWorkflowByID(wf.ID)
				return space, project, fullWorkflow, true
			}
		}
	} else {
		if !silent {
			fmt.Println("No workflows found in " + pathSplit[0] + "/" + pathSplit[1])
		}
		return space, project, nil, false
	}

	if !silent {
		fmt.Println("Couldn't find workflow named " + pathSplit[2] + " in " + pathSplit[0] + "/" + pathSplit[1] + "/")
	}
	return space, project, nil, false
}

func ResolveObjectURL(objectURL string) (*types.SpaceDetailed, *types.Project, *types.Workflow, bool) {
	u, err := url.Parse(objectURL)
	if err != nil {
		fmt.Println("Invalid URL")
		return nil, nil, nil, false
	}

	path := u.Path
	pathSegments := strings.Split(path, "/")

	switch {
	case strings.HasPrefix(path, "/dashboard/spaces"):
		return resolveSpaceURL(pathSegments)
	case strings.HasPrefix(path, "/dashboard/projects"):
		return resolveProjectURL(pathSegments)
	case strings.HasPrefix(path, "/editor/"):
		return resolveWorkflowURL(pathSegments)
	default:
		fmt.Println("Please enter a workflow, project, or space URL")
	}

	return nil, nil, nil, false
}

func resolveSpaceURL(pathSegments []string) (*types.SpaceDetailed, *types.Project, *types.Workflow, bool) {
	spaceID := pathSegments[3]
	spaceUUID, err := uuid.Parse(spaceID)
	if err != nil {
		fmt.Println("Invalid space ID")
		return nil, nil, nil, false
	}

	space := getSpaceByID(spaceUUID)
	return space, nil, nil, true
}

func resolveProjectURL(pathSegments []string) (*types.SpaceDetailed, *types.Project, *types.Workflow, bool) {
	projectID := pathSegments[3]
	projectUUID, err := uuid.Parse(projectID)
	if err != nil {
		fmt.Println("Invalid project ID")
		return nil, nil, nil, false
	}

	project := getProjectByID(projectUUID)
	return nil, project, nil, true
}

func resolveWorkflowURL(pathSegments []string) (*types.SpaceDetailed, *types.Project, *types.Workflow, bool) {
	workflowID := pathSegments[2]
	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		fmt.Println("Invalid workflow ID")
		return nil, nil, nil, false
	}

	workflow := GetWorkflowByID(workflowUUID)
	return nil, nil, workflow, true
}

// GetObjects handles different input scenarios for retrieving platform objects.
//
// Examples:
//   - trickest get space_name/project_name/workflow_name
//   - trickest get --space space_name --project project_name --workflow workflow_name
//   - trickest get --url https://trickest.io/editor/00000000-0000-0000-0000-000000000000
func GetObjects(args []string) (*types.SpaceDetailed, *types.Project, *types.Workflow, bool) {
	path := FormatPath()
	if len(args) > 0 {
		path = strings.Trim(args[0], "/")
	}

	switch {
	case path != "":
		return ResolveObjectPath(path, true, false)

	case URL != "":
		return ResolveObjectURL(URL)

	default:
		fmt.Println("Please specify a path, platform object parameters, or a platform object URL")
		return nil, nil, nil, false
	}
}

func FormatDuration(duration time.Duration) string {
	duration = duration.Round(time.Second)
	units := durafmt.Units{
		Year:   durafmt.Unit{Singular: "year", Plural: "years"},
		Week:   durafmt.Unit{Singular: "week", Plural: "weeks"},
		Day:    durafmt.Unit{Singular: "day", Plural: "days"},
		Hour:   durafmt.Unit{Singular: "h", Plural: "h"},
		Minute: durafmt.Unit{Singular: "m", Plural: "m"},
		Second: durafmt.Unit{Singular: "s", Plural: "s"},
	}

	str := durafmt.Parse(duration).LimitFirstN(2).Format(units)
	str = strings.Replace(str, " s", "s", 1)
	str = strings.Replace(str, " m", "m", 1)
	str = strings.Replace(str, " h", "h", 1)

	return str
}

func DownloadFile(url, outputDir, fileName string) error {
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("couldn't create output directory (%s): %w", outputDir, err)
	}

	filePath := path.Join(outputDir, fileName)
	outputFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("couldn't create output file (%s): %w", filePath, err)
	}
	defer outputFile.Close()

	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("couldn't get URL (%s): %w", url, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status code: %s %d", url, response.StatusCode)
	}

	if response.ContentLength > 0 {
		bar := progressbar.NewOptions64(
			response.ContentLength,
			progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s... ", fileName)),
			progressbar.OptionSetWidth(30),
			progressbar.OptionShowBytes(true),
			progressbar.OptionShowCount(),
			progressbar.OptionOnCompletion(func() { fmt.Print("\n\n") }),
		)
		_, err = io.Copy(io.MultiWriter(outputFile, bar), response.Body)
	} else {
		_, err = io.Copy(outputFile, response.Body)
	}
	if err != nil {
		return fmt.Errorf("couldn't save file content to %s: %w", filePath, err)
	}
	return nil
}
