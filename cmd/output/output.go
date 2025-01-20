package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/trickest/trickest-cli/client/request"
	"github.com/trickest/trickest-cli/types"
	"github.com/trickest/trickest-cli/util"

	"github.com/google/uuid"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

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

		var nodes []string
		if nodesFlag != "" {
			for _, node := range strings.Split(nodesFlag, ",") {
				nodes = append(nodes, strings.ReplaceAll(node, "/", "-"))
			}
		} else if util.URL != "" {
			node, err := util.GetNodeIDFromWorkflowURL(util.URL)
			if err == nil {
				nodes = append(nodes, node)
			}
		}

		var files []string
		if filesFlag != "" {
			files = append(files, strings.Split(filesFlag, ",")...)
		}

		if configFile != "" {
			file, err := os.Open(configFile)
			if err != nil {
				fmt.Println("Couldn't open config file to read outputs!")
				return
			}

			bytes, err := io.ReadAll(file)
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
				nodes = append(nodes, strings.ReplaceAll(node, "/", "-"))
			}
		}

		runs, err := getRelevantRuns(*workflow, allRuns, runID, numberOfRuns, util.URL)
		if err != nil {
			fmt.Printf("Couldn't get workflow runs: %v\n", err)
			return
		}
		if len(runs) == 0 {
			fmt.Printf("No runs found for the workflow: %s (%s)\n", workflow.Name, workflow.ID)
			return
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

func DownloadRunOutput(run *types.Run, nodes []string, files []string, destinationPath string) {
	if run.Status == "PENDING" || run.Status == "SUBMITTED" {
		fmt.Println("The workflow run hasn't started yet!")
		fmt.Println("Run ID: " + run.ID.String() + "   Status: " + run.Status)
		return
	}

	version := util.GetWorkflowVersionByID(*run.WorkflowVersionInfo, uuid.Nil)

	subJobs := util.GetSubJobs(*run.ID)
	subJobs = util.LabelSubJobs(subJobs, *version)

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
			isModule := false
			if (version.Data.Nodes[subJob.Name]).Type == "WORKFLOW" {
				isModule = true
			}
			err = downloadSubJobOutput(runDir, &subJob, files, run.ID, isModule)
			if err != nil {
				fmt.Printf("Error downloading output for node %s: %v\n", subJob.Label, err)
			}
		}
	} else {
		noneFound := true
		var foundNodes []string
		for _, subJob := range subJobs {
			labelExists := slices.Contains(nodes, subJob.Label)
			if labelExists {
				foundNodes = append(foundNodes, subJob.Label)
			}
			nameExists := slices.Contains(nodes, subJob.Name)
			if nameExists {
				foundNodes = append(foundNodes, subJob.Name)
			}
			if nameExists || labelExists {
				noneFound = false
				isModule := false
				if (version.Data.Nodes[subJob.Name]).Type == "WORKFLOW" {
					isModule = true
				}
				err = downloadSubJobOutput(runDir, &subJob, files, run.ID, isModule)
				if err != nil {
					fmt.Printf("Error downloading output for node %s: %v\n", subJob.Label, err)
				}
			}
		}
		if noneFound {
			runURL := fmt.Sprintf("https://trickest.io/editor/%s?run=%s", run.WorkflowInfo, run.ID)
			fmt.Printf("No completed node outputs matching your query were found in the \"%s\" run: %s\n", run.StartedDate.Format(layout), runURL)
		} else {
			for _, node := range nodes {
				if !slices.Contains(foundNodes, node) {
					fmt.Println("Couldn't find any sub-job named " + node + "!")
				}
			}
		}
	}
}

func getRelevantRuns(workflow types.Workflow, allRuns bool, runID string, numberOfRuns int, workflowURL string) ([]types.Run, error) {
	switch {
	case allRuns:
		return util.GetRuns(workflow.ID, math.MaxInt, ""), nil

	case runID != "":
		runUUID, err := uuid.Parse(runID)
		if err != nil {
			return nil, fmt.Errorf("invalid run ID: %s", runID)
		}
		run := util.GetRunByID(runUUID)
		return []types.Run{*run}, nil

	case numberOfRuns > 1:
		return util.GetRuns(workflow.ID, numberOfRuns, ""), nil

	default:
		workflowURLRunID, _ := util.GetRunIDFromWorkflowURL(workflowURL)
		if runUUID, err := uuid.Parse(workflowURLRunID); err == nil {
			run := util.GetRunByID(runUUID)
			return []types.Run{*run}, nil
		}
		return util.GetRuns(workflow.ID, 1, ""), nil
	}
}

func getSubJobOutputs(subJob types.SubJob, runID uuid.UUID, isModule bool) ([]types.SubJobOutput, error) {
	urlReq := "subjob-output/?subjob=" + subJob.ID.String()
	if isModule {
		urlReq = fmt.Sprintf("subjob-output/module-outputs/?module_name=%s&execution=%s", subJob.Name, runID.String())
	}

	urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)

	resp := request.Trickest.Get().DoF(urlReq)
	if resp == nil {
		return nil, fmt.Errorf("couldn't get sub-job output data for sub-job %s: empty response", subJob.Label)
	}

	if resp.Status() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status code for sub-job %s: %d", subJob.Label, resp.Status())
	}

	var subJobOutputs types.SubJobOutputs
	err := json.Unmarshal(resp.Body(), &subJobOutputs)
	if err != nil {
		return nil, fmt.Errorf("couldn't unmarshal sub-job output response for sub-job %s: %v", subJob.Label, err)
	}

	if subJobOutputs.Count == 0 {
		return nil, fmt.Errorf("no output files found for sub-job %s", subJob.Label)
	}

	return subJobOutputs.Results, nil
}

func getOutputSignedURL(outputID uuid.UUID) (string, error) {
	resp := request.Trickest.Get().DoF("subjob-output/%s/signed_url/", outputID)
	if resp == nil {
		return "", fmt.Errorf("couldn't get output signed URL for output %s", outputID)
	}

	if resp.Status() != http.StatusOK {
		return "", fmt.Errorf("unexpected response status code for output %s: %d", outputID, resp.Status())
	}

	var signedURL types.SignedURL
	err := json.Unmarshal(resp.Body(), &signedURL)
	if err != nil {
		return "", fmt.Errorf("couldn't unmarshal output signed URL response for output %s: %v", outputID, err)
	}

	return signedURL.Url, nil
}

func createSubJobOutputFile(savePath string, output types.SubJobOutput) (*os.File, error) {
	if output.Path != "" {
		savePath = path.Join(savePath, output.Path)
	}

	err := os.MkdirAll(savePath, 0755)
	if err != nil {
		return nil, fmt.Errorf("couldn't create a directory to store output: %v", err)
	}

	filePath := path.Join(savePath, output.Name)
	outputFile, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("couldn't create a file to store output: %v", err)
	}
	return outputFile, nil
}

func downloadSubJobOutput(savePath string, subJob *types.SubJob, files []string, runID *uuid.UUID, isModule bool) error {
	var errs []error

	if !subJob.TaskGroup {
		if subJob.Status != "SUCCEEDED" {
			return fmt.Errorf("sub-job %s is not in a completed state: %s", subJob.Label, subJob.Status)
		}
	}

	savePath = path.Join(savePath, subJob.Label)
	if subJob.TaskGroup {
		subJobCount, err := util.GetChildrenSubJobsCount(*subJob)
		if err != nil {
			return fmt.Errorf("couldn't get children sub-jobs count for sub-job %s", subJob.Label)
		}
		if subJobCount == 0 {
			return fmt.Errorf("no children sub-jobs found for sub-job %s", subJob.Label)
		}

		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 5)

		for i := 1; i <= subJobCount; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				child, err := util.GetChildSubJob(subJob.ID, i)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("couldn't get child %d sub-jobs for sub-job %s: %v", i, subJob.Label, err))
					mu.Unlock()
					return
				}
				child.Label = fmt.Sprintf("%d-%s", i, subJob.Label)
				err = downloadSubJobOutput(savePath, &child, files, runID, false)
				if err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
				}
			}(i)
		}
		wg.Wait()

		if len(errs) > 0 {
			return fmt.Errorf("errors occurred while downloading sub-job children outputs:\n%s", errors.Join(errs...))
		}
		return nil
	}

	subJobOutputs, err := getSubJobOutputs(*subJob, *runID, isModule)
	if err != nil {
		return fmt.Errorf("couldn't get sub-job outputs for node %s: %v", subJob.Label, err)
	}
	subJobOutputs = filterSubJobOutputsByFileNames(subJobOutputs, files)
	if len(subJobOutputs) == 0 {
		return fmt.Errorf("no matching output files found for node %s", subJob.Label)
	}

	for _, output := range subJobOutputs {
		signedURL, err := getOutputSignedURL(output.ID)
		if err != nil {
			errs = append(errs, fmt.Errorf("couldn't get signed URL for output %s of node %s: %v", output.Name, subJob.Label, err))
			continue
		}
		outputFile, err := createSubJobOutputFile(savePath, output)
		if err != nil {
			errs = append(errs, fmt.Errorf("couldn't create a file to store output %s: %v", output.Name, err))
			continue
		}
		defer outputFile.Close()

		err = downloadFile(signedURL, outputFile, subJob.Label)
		if err != nil {
			errs = append(errs, fmt.Errorf("couldn't download file for output %s of node %s: %v", output.Name, subJob.Label, err))
			continue
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred while downloading sub-job outputs:\n%s", errors.Join(errs...))
	}

	return nil
}

func downloadFile(url string, outputFile *os.File, label string) error {
	dataResp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("couldn't fetch output data: %v", err)
	}
	defer dataResp.Body.Close()

	if dataResp.StatusCode != http.StatusOK {
		return fmt.Errorf("couldn't download output for %s! HTTP status code: %d", label, dataResp.StatusCode)
	}

	if dataResp.ContentLength > 0 {
		bar := progressbar.NewOptions64(
			dataResp.ContentLength,
			progressbar.OptionSetDescription(fmt.Sprintf("Downloading [%s] output to %s", label, outputFile.Name())),
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
		return fmt.Errorf("couldn't save data: %v", err)
	}

	return nil
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
