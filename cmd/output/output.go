package output

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"

	"github.com/google/uuid"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type NodeInfo struct {
	ToFetch bool
	Found   bool
}

type LabelCnt struct {
	name string
	cnt  int
}

var (
	configFile   string
	allRuns      bool
	numberOfRuns int
	runID        string
	outputDir    string
	nodesFlag    string
	filesFlag    string
)

// OutputCmd represents the download command
var OutputCmd = &cobra.Command{
	Use:   "output",
	Short: "Download workflow outputs",
	Long: `This command downloads sub-job outputs of a completed workflow run.
Downloaded files will be stored into space/project/workflow/run-timestamp directory. Every node will have it's own
directory named after it's label or ID (if the label is not unique), and an optional prefix ("<num>-") if it's 
connected to a splitter.

Use raw command line arguments or a config file to specify which nodes' output you would like to fetch.
If there is no node names specified, all outputs will be downloaded.

The YAML config file should be formatted like:
   outputs:
      - foo
      - bar
`,
	Run: func(cmd *cobra.Command, args []string) {
		_, _, workflow, found := util.GetObjects(args)
		if !found {
			return
		}

		nodes := make(map[string]NodeInfo, 0)
		if nodesFlag != "" {
			for _, node := range strings.Split(nodesFlag, ",") {
				nodes[strings.ReplaceAll(node, "/", "-")] = NodeInfo{ToFetch: true, Found: false}
			}
		}

		var files []string
		if filesFlag != "" {
			for _, file := range strings.Split(filesFlag, ",") {
				files = append(files, file)
			}
		}

		if configFile != "" {
			file, err := os.Open(configFile)
			if err != nil {
				fmt.Println("Couldn't open config file to read outputs!")
				return
			}

			bytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println("Couldn't read outputs config!")
				return
			}

			var conf types.OutputsConfig
			err = yaml.Unmarshal(bytes, &conf)
			if err != nil {
				fmt.Println("Couldn't unmarshal outputs config!")
				return
			}

			for _, node := range conf.Outputs {
				nodes[strings.ReplaceAll(node, "/", "-")] = NodeInfo{ToFetch: true, Found: false}
			}
		}

		runs := make([]types.Run, 0)

		if runID == "" && util.URL != "" {
			workflowURLRunID, err := util.GetRunIDFromWorkflowURL(util.URL)
			if err == nil {
				runID = workflowURLRunID
			}
		}
		if allRuns {
			numberOfRuns = math.MaxInt
		}
		if runID == "" {
			wfRuns := GetRuns(workflow.ID, numberOfRuns)
			if wfRuns != nil && len(wfRuns) > 0 {
				runs = append(runs, wfRuns...)
			} else {
				fmt.Println("This workflow has not been executed yet!")
				return
			}
		} else {
			runUUID, err := uuid.Parse(runID)
			if err != nil {
				fmt.Println("Invalid run ID")
				return
			}
			run := GetRunByID(runUUID)
			runs = []types.Run{*run}
		}

		if numberOfRuns == 1 && runs[0].Status == "SCHEDULED" {
			runs = GetRuns(workflow.ID, numberOfRuns+1)
			runs = append(runs, runs...)
		}

		path := util.FormatPath()
		if outputDir != "" {
			path = outputDir
		}
		for _, run := range runs {
			if run.Status == "SCHEDULED" {
				continue
			}
			DownloadRunOutput(&run, nodes, files, path)
		}
	},
}

func init() {
	OutputCmd.Flags().StringVar(&configFile, "config", "", "YAML file to determine which nodes output(s) should be downloaded")
	OutputCmd.Flags().BoolVar(&allRuns, "all", false, "Download output data for all runs")
	OutputCmd.Flags().IntVar(&numberOfRuns, "runs", 1, "Number of recent runs which outputs should be downloaded")
	OutputCmd.Flags().StringVar(&runID, "run", "", "Download output data of a specific run")
	OutputCmd.Flags().StringVar(&outputDir, "output-dir", "", "Path to directory which should be used to store outputs")
	OutputCmd.Flags().StringVar(&nodesFlag, "nodes", "", "A comma-separated list of nodes whose outputs should be downloaded")
	OutputCmd.Flags().StringVar(&filesFlag, "files", "", "A comma-separated list of file names that should be downloaded from the selected node")
}

func DownloadRunOutput(run *types.Run, nodes map[string]NodeInfo, files []string, destinationPath string) {
	if run.Status != "COMPLETED" && run.Status != "STOPPED" && run.Status != "FAILED" {
		fmt.Println("The workflow run hasn't been completed yet!")
		fmt.Println("Run ID: " + run.ID.String() + "   Status: " + run.Status)
		return
	}

	version := GetWorkflowVersionByID(*run.WorkflowVersionInfo, uuid.Nil)

	subJobs := getSubJobs(*run.ID)
	labels := make(map[string]bool)

	for i := range subJobs {
		subJobs[i].Label = version.Data.Nodes[subJobs[i].Name].Meta.Label
		subJobs[i].Label = strings.ReplaceAll(subJobs[i].Label, "/", "-")
		if labels[subJobs[i].Label] {
			existingLabel := subJobs[i].Label
			subJobs[i].Label = subJobs[i].Name
			if labels[subJobs[i].Label] {
				subJobs[i].Label += "-1"
				for c := 1; c >= 1; c++ {
					if labels[subJobs[i].Label] {
						subJobs[i].Label = strings.TrimSuffix(subJobs[i].Label, "-"+strconv.Itoa(c))
						subJobs[i].Label += "-" + strconv.Itoa(c+1)
					} else {
						labels[subJobs[i].Label] = true
						break
					}
				}
			} else {
				for s := 0; s < i; s++ {
					if subJobs[s].Label == existingLabel {
						subJobs[s].Label = subJobs[s].Name
						if subJobs[s].Children != nil {
							for j := range subJobs[s].Children {
								subJobs[s].Children[j].Label = strconv.Itoa(subJobs[s].Children[j].TaskIndex) + "-" + subJobs[s].Name
							}
						}
					}
				}
				labels[subJobs[i].Label] = true
			}
		} else {
			labels[subJobs[i].Label] = true
		}
	}

	const layout = "2006-01-02T150405Z"
	runDir := "run-" + run.StartedDate.Format(layout)
	runDir = strings.TrimSuffix(runDir, "Z")
	runDir = strings.Replace(runDir, "T", "-", 1)
	runDir = path.Join(destinationPath, runDir)

	err := os.MkdirAll(runDir, 0755)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Couldn't create a directory to store run output!")
		os.Exit(0)
	}

	if len(nodes) == 0 {
		for _, subJob := range subJobs {
			for subJob.OutputsStatus == "SAVING" || subJob.OutputsStatus == "WAITING" {
				updatedSubJob := getSubJobByID(subJob.ID)
				if updatedSubJob == nil {
					os.Exit(0)
				}
				subJob.OutputsStatus = updatedSubJob.OutputsStatus
			}
			getSubJobOutput(runDir, &subJob, files, true)
		}
	} else {
		noneFound := true
		for _, subJob := range subJobs {
			_, labelExists := nodes[subJob.Label]
			if labelExists {
				nodes[subJob.Label] = NodeInfo{ToFetch: true, Found: true}
			}
			_, nameExists := nodes[subJob.Name]
			if nameExists {
				nodes[subJob.Name] = NodeInfo{ToFetch: true, Found: true}
			}
			_, nodeIDExists := nodes[subJob.Name]
			if nodeIDExists {
				nodes[subJob.Name] = NodeInfo{ToFetch: true, Found: true}
			}
			if nameExists || labelExists || nodeIDExists {
				noneFound = false
				for subJob.OutputsStatus == "SAVING" || subJob.OutputsStatus == "WAITING" {
					updatedSubJob := getSubJobByID(subJob.ID)
					if updatedSubJob == nil {
						os.Exit(0)
					}
					subJob.OutputsStatus = updatedSubJob.OutputsStatus
				}
				getSubJobOutput(runDir, &subJob, files, true)
			}
		}
		if noneFound {
			fmt.Printf("No completed node outputs matching your query were found in the \"%s\" run.", run.StartedDate.Format(layout))
		} else {
			for nodeName, nodeInfo := range nodes {
				if !nodeInfo.Found {
					fmt.Println("Couldn't find any sub-job named " + nodeName + "!")
				}
			}
		}
	}
}

func getSubJobOutput(savePath string, subJob *types.SubJob, files []string, fetchData bool) []types.SubJobOutput {
	if subJob.Status != "SUCCEEDED" {
		if subJob.TaskGroup && subJob.Status != "STOPPED" {
			return nil
		}
	}

	urlReq := "subjob-output/?subjob=" + subJob.ID.String()
	urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get sub-job output data.")
		return nil
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var subJobOutputs types.SubJobOutputs
	err := json.Unmarshal(resp.Body(), &subJobOutputs)
	if err != nil {
		fmt.Println("Error unmarshalling sub-job output response!")
		return nil
	}

	if subJob.TaskGroup {
		savePath = path.Join(savePath, subJob.Label)
		dirInfo, err := os.Stat(savePath)
		dirExists := !os.IsNotExist(err) && dirInfo.IsDir()

		if !dirExists {
			err = os.Mkdir(savePath, 0755)
			if err != nil {
				fmt.Println("Couldn't create a directory to store multiple outputs for " + subJob.Label + "!")
				os.Exit(0)
			}
		}

		children := getChildrenSubJobs(subJob.ID)
		if children == nil {
			return nil
		}
		for j := range children {
			children[j].Label = fmt.Sprint(j) + "-" + subJob.Label
		}

		subJob.Children = make([]types.SubJob, 0)
		subJob.Children = append(subJob.Children, children...)

		results := make([]types.SubJobOutput, 0)
		if subJob.Children != nil {
			for _, child := range subJob.Children {
				childRes := getSubJobOutput(savePath, &child, files, true)
				if childRes != nil {
					results = append(results, childRes...)
				}
			}
		}
		return results
	}

	dir := subJob.Label
	savePath = path.Join(savePath, dir)
	dirInfo, err := os.Stat(savePath)
	dirExists := !os.IsNotExist(err) && dirInfo.IsDir()

	if !dirExists {
		err = os.Mkdir(savePath, 0755)
		if err != nil {
			fmt.Println("Couldn't create a directory to store outputs for " + subJob.Label + "!")
			os.Exit(0)
		}
	}

	subJobOutputResults := filterSubJobOutputsByFileNames(subJobOutputs.Results, files)
	for i, output := range subJobOutputResults {
		resp := request.Trickest.Get().DoF("subjob-output/%s/signed_url/", output.ID)
		if resp == nil {
			fmt.Println("Error: Couldn't get sub-job outputs signed URL.")
			continue
		}

		if resp.Status() != http.StatusNotFound && resp.Status() != http.StatusOK {
			request.ProcessUnexpectedResponse(resp)
		}

		var signedURL types.SignedURL
		err = json.Unmarshal(resp.Body(), &signedURL)
		if err != nil {
			fmt.Println("Error unmarshalling sub-job output signed URL response!")
			continue
		}

		if resp.Status() == http.StatusNotFound {
			subJobOutputResults[i].SignedURL = "expired"
		} else {
			subJobOutputResults[i].SignedURL = signedURL.Url

			if fetchData {
				fileName := subJobOutputResults[i].Name

				if subJobOutputResults[i].Path != "" {
					subDirsPath := path.Join(savePath, subJobOutputResults[i].Path)
					err := os.MkdirAll(subDirsPath, 0755)
					if err != nil {
						fmt.Println(err)
						fmt.Println("Couldn't create a directory to store run output!")
						os.Exit(0)
					}
					fileName = path.Join(subDirsPath, fileName)
				} else {
					fileName = path.Join(savePath, fileName)
				}

				outputFile, err := os.Create(fileName)
				if err != nil {
					fmt.Println(err)
					fmt.Println("Couldn't create file to store data!")
					continue
				}

				dataResp, err := http.Get(signedURL.Url)
				if err != nil {
					fmt.Println("Couldn't fetch output data!")
					continue
				}

				if dataResp.StatusCode != http.StatusOK {
					fmt.Println("Couldn't download output for " + subJob.Label +
						"! HTTP status code: " + strconv.Itoa(dataResp.StatusCode))
					continue
				}

				if dataResp.ContentLength > 0 {
					bar := progressbar.NewOptions64(
						dataResp.ContentLength,
						progressbar.OptionSetDescription("Downloading ["+subJob.Label+"] output... "),
						progressbar.OptionSetWidth(30),
						progressbar.OptionShowBytes(true),
						progressbar.OptionShowCount(),
						progressbar.OptionOnCompletion(func() { fmt.Print("\n\n") }),
					)
					_, err = io.Copy(io.MultiWriter(outputFile, bar), dataResp.Body)
				} else {
					_, err = io.Copy(outputFile, dataResp.Body)
				}
				if err != nil {
					fmt.Println("Couldn't save data!")
					continue
				}

				_ = outputFile.Close()
				_ = dataResp.Body.Close()
				if dataResp.ContentLength > 0 {
					fmt.Println()
				}
			}
		}
	}

	return subJobOutputResults
}

func filterSubJobOutputsByFileNames(outputs []types.SubJobOutput, fileNames []string) []types.SubJobOutput {
	if fileNames == nil {
		return outputs
	}

	var matchingOutputs []types.SubJobOutput
	for _, output := range outputs {
		for _, fileName := range fileNames {
			if output.Name == fileName {
				matchingOutputs = append(matchingOutputs, output)
				break
			}
		}
	}

	return matchingOutputs
}

func GetRunByID(id uuid.UUID) *types.Run {
	resp := request.Trickest.Get().DoF("execution/%s/", id)
	if resp == nil {
		fmt.Println("Error: Couldn't get run!")
		os.Exit(0)
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var run types.Run
	err := json.Unmarshal(resp.Body(), &run)
	if err != nil {
		fmt.Println("Error unmarshalling run response!")
		return nil
	}

	return &run
}

func getSubJobs(runID uuid.UUID) []types.SubJob {
	if runID == uuid.Nil {
		fmt.Println("Couldn't list sub-jobs, no run ID parameter specified!")
		return nil
	}
	urlReq := "subjob/?execution=" + runID.String()
	urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get sub-jobs!")
		return nil
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var subJobs types.SubJobs
	err := json.Unmarshal(resp.Body(), &subJobs)
	if err != nil {
		fmt.Println("Error unmarshalling sub-jobs response!")
		return nil
	}

	return subJobs.Results
}

func GetRuns(workflowID uuid.UUID, pageSize int) []types.Run {
	urlReq := "execution/?type=Editor&vault=" + util.GetVault().String()

	if workflowID != uuid.Nil {
		urlReq += "&workflow=" + workflowID.String()
	}

	if pageSize != 0 {
		urlReq += "&page_size=" + strconv.Itoa(pageSize)
	} else {
		urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)
	}

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get runs!")
		return nil
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var runs types.Runs
	err := json.Unmarshal(resp.Body(), &runs)
	if err != nil {
		fmt.Println("Error unmarshalling runs response!")
		return nil
	}

	return runs.Results
}

func GetWorkflowVersionByID(versionID, fleetID uuid.UUID) *types.WorkflowVersionDetailed {
	resp := request.Trickest.Get().DoF("workflow-version/%s/", versionID)
	if resp == nil {
		fmt.Println("Error: Couldn't get workflow version!")
		return nil
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var workflowVersion types.WorkflowVersionDetailed
	err := json.Unmarshal(resp.Body(), &workflowVersion)
	if err != nil {
		fmt.Println("Error unmarshalling workflow version response!")
		return nil
	}

	if fleetID != uuid.Nil {
		maxMachines, err := GetWorkflowVersionMaxMachines(versionID, fleetID)
		if err != nil {
			fmt.Printf("Error getting maximum machines: %v", err)
			return nil
		}
		workflowVersion.MaxMachines = maxMachines

	}
	return &workflowVersion
}

func GetWorkflowVersionMaxMachines(version, fleet uuid.UUID) (types.Machines, error) {
	resp := request.Trickest.Get().DoF("workflow-version/%s/max-machines/?fleet=%s", version, fleet)
	if resp == nil {
		return types.Machines{}, fmt.Errorf("couldn't get workflow version's maximum machines")
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var machines types.Machines
	err := json.Unmarshal(resp.Body(), &machines)
	if err != nil {
		return types.Machines{}, fmt.Errorf("couldn't unmarshal workflow versions's maximum machines: %v", err)
	}

	return machines, nil
}

func getChildrenSubJobsCount(subJobID uuid.UUID) int {
	urlReq := "subjob/children/?parent=" + subJobID.String()
	urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		fmt.Println("Error: Couldn't get children sub-jobs!")
		return 0
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var subJobs types.SubJobs
	err := json.Unmarshal(resp.Body(), &subJobs)
	if err != nil {
		fmt.Println("Error unmarshalling sub-job children response!")
		return 0
	}

	return subJobs.Count
}

func getChildrenSubJobs(subJobID uuid.UUID) []types.SubJob {
	subJobCount := getChildrenSubJobsCount(subJobID)
	if subJobCount == 0 {
		fmt.Println("Error: Couldn't find children sub-jobs!")
		return nil
	}

	var subJobs []types.SubJob

	urlReq := "subjob/children/?parent=" + subJobID.String()
	urlReq += "&task_index="

	for i := 1; i <= subJobCount; i++ {
		urlReqForIndex := urlReq + strconv.Itoa(i)
		resp := request.Trickest.Get().DoF(urlReqForIndex)
		if resp == nil {
			fmt.Printf("Error: Couldn't get child sub-job: %d", i)
			continue
		}
		if resp.Status() != http.StatusOK {
			request.ProcessUnexpectedResponse(resp)
		}

		var child types.SubJobs

		err := json.Unmarshal(resp.Body(), &child)
		if err != nil {
			fmt.Println("Error unmarshalling sub-job child response!")
			continue
		}

		if len(child.Results) < 1 {
			fmt.Println("Error: Unexpected sub-job child response!")
			continue
		}
		subJobs = append(subJobs, child.Results...)
	}

	return subJobs
}

func getSubJobByID(id uuid.UUID) *types.SubJob {
	resp := request.Trickest.Get().DoF("subjob/%s/", id)
	if resp == nil {
		fmt.Println("Error: Couldn't get sub-job!")
		return nil
	}

	if resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var subJob types.SubJob
	err := json.Unmarshal(resp.Body(), &subJob)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return &subJob
}
