package output

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"trickest-cli/cmd/list"
	"trickest-cli/types"
	"trickest-cli/util"

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
		nodes := make(map[string]NodeInfo, 0)

		path := util.FormatPath()
		if path == "" {
			if len(args) == 0 {
				fmt.Println("Workflow path must be specified!")
				return
			}
			path = strings.Trim(args[0], "/")
			if len(args) > 1 {
				for i := 1; i < len(args); i++ {
					nodes[strings.ReplaceAll(args[i], "/", "-")] = NodeInfo{ToFetch: true, Found: false}
				}
			}
		} else {
			if util.WorkflowName == "" {
				fmt.Println("Workflow must be specified!")
				return
			}
			if len(args) > 0 {
				for i := 0; i < len(args); i++ {
					nodes[strings.ReplaceAll(args[i], "/", "-")] = NodeInfo{ToFetch: true, Found: false}
				}
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

		_, _, workflow, found := list.ResolveObjectPath(path, false, false)
		if !found {
			return
		}

		runs := make([]types.Run, 0)

		if allRuns {
			numberOfRuns = math.MaxInt
		}
		wfRuns := GetRuns(workflow.ID, numberOfRuns)
		if wfRuns != nil && len(wfRuns) > 0 {
			runs = append(runs, wfRuns...)
		} else {
			fmt.Println("This workflow has not been executed yet!")
			return
		}

		if numberOfRuns == 1 && (wfRuns[0].Status == "SCHEDULED" || wfRuns[0].CreationType == types.RunCreationScheduled) {
			wfRuns = GetRuns(workflow.ID, numberOfRuns+1)
			runs = append(runs, wfRuns...)
		}

		version := GetWorkflowVersionByID(runs[0].WorkflowVersionInfo)
		if version == nil {
			return
		}

		for _, run := range runs {
			if run.Status == "SCHEDULED" {
				continue
			}
			DownloadRunOutput(&run, nodes, version, path)
		}
	},
}

func init() {
	OutputCmd.Flags().StringVar(&configFile, "config", "", "YAML file to determine which nodes output(s) should be downloaded")
	OutputCmd.Flags().BoolVar(&allRuns, "all", false, "Download output data for all runs")
	OutputCmd.Flags().IntVar(&numberOfRuns, "runs", 1, "Number of recent runs which outputs should be downloaded")
}

func DownloadRunOutput(run *types.Run, nodes map[string]NodeInfo, version *types.WorkflowVersionDetailed, destinationPath string) {
	if run.Status != "COMPLETED" && run.Status != "STOPPED" && run.Status != "FAILED" {
		fmt.Println("The workflow run hasn't been completed yet!")
		fmt.Println("Run ID: " + run.ID.String() + "   Status: " + run.Status)
		return
	}

	if version == nil {
		version = GetWorkflowVersionByID(run.WorkflowVersionInfo)
	}

	subJobs := getSubJobs(run.ID)
	labels := make(map[string]bool)

	for i := range subJobs {
		subJobs[i].Label = version.Data.Nodes[subJobs[i].NodeName].Meta.Label
		subJobs[i].Label = strings.ReplaceAll(subJobs[i].Label, "/", "-")
		if labels[subJobs[i].Label] {
			existingLabel := subJobs[i].Label
			subJobs[i].Label = subJobs[i].NodeName
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
						subJobs[s].Label = subJobs[s].NodeName
						if subJobs[s].Children != nil {
							for j := range subJobs[s].Children {
								subJobs[s].Children[j].Label = subJobs[s].Children[j].TaskIndex + "-" + subJobs[s].NodeName
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
	runDirPath := strings.Split(runDir, "/")
	toMerge := ""
	for _, dir := range runDirPath {
		toMerge = path.Join(toMerge, dir)
		dirInfo, err := os.Stat(toMerge)
		dirExists := !os.IsNotExist(err) && dirInfo.IsDir()

		if !dirExists {
			err = os.Mkdir(toMerge, 0755)
			if err != nil {
				fmt.Println(err)
				fmt.Println("Couldn't create a directory to store run output!")
				os.Exit(0)
			}
		}
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
			getSubJobOutput(runDir, &subJob, true)
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
			_, nodeIDExists := nodes[subJob.NodeName]
			if nodeIDExists {
				nodes[subJob.NodeName] = NodeInfo{ToFetch: true, Found: true}
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
				getSubJobOutput(runDir, &subJob, true)
			}
		}
		if noneFound {
			fmt.Println("Couldn't find any nodes that match given name(s)!")
		} else {
			for nodeName, nodeInfo := range nodes {
				if !nodeInfo.Found {
					fmt.Println("Couldn't find any sub-job named " + nodeName + "!")
				}
			}
		}
	}
}

func getSubJobOutput(savePath string, subJob *types.SubJob, fetchData bool) []types.SubJobOutput {
	if subJob.OutputsStatus != "SAVED" && !subJob.TaskGroup {
		return nil
	}
	client := &http.Client{}

	urlReq := util.Cfg.BaseUrl + "v1/subjob-output/?subjob=" + subJob.ID.String()
	urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)

	req, err := http.NewRequest("GET", urlReq, nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get sub-job output data.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read sub-job output data.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(resp)
	}

	var subJobOutputs types.SubJobOutputs
	err = json.Unmarshal(bodyBytes, &subJobOutputs)
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
		if children == nil || len(children) == 0 {
			return nil
		}
		for j := range children {
			children[j].Label = children[j].TaskIndex + "-" + subJob.Label
		}

		subJob.Children = make([]types.SubJob, 0)
		subJob.Children = append(subJob.Children, children...)

		results := make([]types.SubJobOutput, 0)
		if subJob.Children != nil {
			for _, child := range subJob.Children {
				childRes := getSubJobOutput(savePath, &child, true)
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

	for i, output := range subJobOutputs.Results {
		req, err = http.NewRequest("POST", util.Cfg.BaseUrl+"v1/subjob-output/"+output.ID.String()+"/signed_url/", nil)
		req.Header.Add("Authorization", "Token "+util.GetToken())
		req.Header.Add("Accept", "application/json")

		resp, err = client.Do(req)
		if err != nil {
			fmt.Println("Error: Couldn't get sub-job outputs signed URL.")
			continue
		}

		bodyBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error: Couldn't read sub-job output signed URL.")
			continue
		}

		var signedURL types.SignedURL
		err = json.Unmarshal(bodyBytes, &signedURL)
		if err != nil {
			fmt.Println("Error unmarshalling sub-job output signed URL response!")
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			subJobOutputs.Results[i].SignedURL = "expired"
		} else {
			subJobOutputs.Results[i].SignedURL = signedURL.Url

			if fetchData {
				fileName := subJobOutputs.Results[i].FileName

				if fileName != subJobOutputs.Results[i].Path {
					subDirsPath := strings.TrimSuffix(subJobOutputs.Results[i].Path, fileName)
					subDirs := strings.Split(strings.Trim(subDirsPath, "/"), "/")
					toMerge := savePath
					for _, subDir := range subDirs {
						toMerge = path.Join(toMerge, subDir)
						dirInfo, err := os.Stat(toMerge)
						dirExists := !os.IsNotExist(err) && dirInfo.IsDir()

						if !dirExists {
							err = os.Mkdir(toMerge, 0755)
							if err != nil {
								fmt.Println(err)
								fmt.Println("Couldn't create a directory to store run output!")
								os.Exit(0)
							}
						}
					}
					fileName = subJobOutputs.Results[i].Path
				}

				fileName = path.Join(savePath, fileName)

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

	return subJobOutputs.Results
}

func getSubJobs(runID uuid.UUID) []types.SubJob {
	if runID == uuid.Nil {
		fmt.Println("Couldn't list sub-jobs, no run ID parameter specified!")
		return nil
	}
	urlReq := util.Cfg.BaseUrl + "v1/subjob/?run=" + runID.String()
	urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)

	client := &http.Client{}
	req, err := http.NewRequest("GET", urlReq, nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get sub-jobs!")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read sub-jobs response.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(resp)
	}

	var subJobs types.SubJobs
	err = json.Unmarshal(bodyBytes, &subJobs)
	if err != nil {
		fmt.Println("Error unmarshalling sub-jobs response!")
		return nil
	}

	return subJobs.Results
}

func GetRuns(workflowID uuid.UUID, pageSize int) []types.Run {
	urlReq := util.Cfg.BaseUrl + "v1/run/?vault=" + util.GetVault().String()

	if workflowID != uuid.Nil {
		urlReq += "&workflow=" + workflowID.String()
	}

	if pageSize != 0 {
		urlReq += "&page_size=" + strconv.Itoa(pageSize)
	} else {
		urlReq += "&page_size=" + strconv.Itoa(math.MaxInt)
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", urlReq, nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get runs!")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read runs response.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(resp)
	}

	var runs types.Runs
	err = json.Unmarshal(bodyBytes, &runs)
	if err != nil {
		fmt.Println("Error unmarshalling runs response!")
		return nil
	}

	return runs.Results
}

func GetWorkflowVersionByID(id uuid.UUID) *types.WorkflowVersionDetailed {
	client := &http.Client{}

	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/store/workflow-version/"+id.String()+"/", nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get workflow version.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read workflow version.")
		return nil
	}

	var workflowVersion types.WorkflowVersionDetailed
	err = json.Unmarshal(bodyBytes, &workflowVersion)
	if err != nil {
		fmt.Println("Error unmarshalling workflow version response!")
		return nil
	}

	return &workflowVersion
}

func getChildrenSubJobs(subJobID uuid.UUID) []types.SubJob {
	client := &http.Client{}

	urlReq := util.Cfg.BaseUrl + "v1/subjob/" + subJobID.String() + "/children/"
	urlReq += "?page_size=" + strconv.Itoa(math.MaxInt)

	req, err := http.NewRequest("GET", urlReq, nil)
	req.Header.Add("Authorization", "Token "+util.Cfg.User.Token)
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get sub-job children.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read sub-job children.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(resp)
	}

	var subJobs types.SubJobs
	err = json.Unmarshal(bodyBytes, &subJobs)
	if err != nil {
		fmt.Println("Error unmarshalling sub-job children response!")
		return nil
	}

	return subJobs.Results
}

func getSubJobByID(id uuid.UUID) *types.SubJob {
	client := &http.Client{}

	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/subjob/"+id.String()+"/", nil)
	req.Header.Add("Authorization", "Token "+util.GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get sub-job info.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read sub-job info.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(resp)
	}

	var subJob types.SubJob
	err = json.Unmarshal(bodyBytes, &subJob)
	if err != nil {
		fmt.Println("Error unmarshalling sub-job response!")
		return nil
	}

	return &subJob
}
