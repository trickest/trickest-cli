package download

import (
	"encoding/json"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"trickest-cli/cmd/list"
	"trickest-cli/types"
	"trickest-cli/util"
)

type NodeInfo struct {
	toFetch bool
	found   bool
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

// DownloadCmd represents the download command
var DownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download workflow outputs",
	Long: `This command downloads sub-job outputs of a completed workflow run.
Downloaded file names will consist of the sub-job name, a timestamp when the sub-job has been completed,
and the name of the actual file stored on the platform. If there are multiple output files for a certain sub-job,
all of them will be stored in a single directory.

Use basic command line arguments or a config file to specify which nodes' output you would like to fetch.
If there is no node names specified, all outputs will be downloaded.

The YAML config file should be formatted like:
   outputs:
      - foo
      - bar
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Workflow path must be specified!")
			return
		}

		nodes := make(map[string]NodeInfo, 0)
		if len(args) > 1 {
			for i := 1; i < len(args); i++ {
				nodes[strings.ReplaceAll(args[i], "/", "-")] = NodeInfo{toFetch: true, found: false}
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
				nodes[strings.ReplaceAll(node, "/", "-")] = NodeInfo{toFetch: true, found: false}
			}
		}

		_, _, workflow := list.ResolveObjectPath(args[0])
		if workflow == nil {
			return
		}

		runs := make([]types.Run, 0)

		if allRuns {
			numberOfRuns = math.MaxInt
		}
		wfRuns := getRuns(workflow.ID, numberOfRuns)
		if wfRuns != nil && len(wfRuns) > 0 {
			runs = append(runs, wfRuns...)
		} else {
			fmt.Println("This workflow has not been executed yet!")
			return
		}

		version := getWorkflowVersionByID(runs[0].WorkflowVersionInfo)
		if version == nil {
			return
		}

		for _, run := range runs {
			if run.Status != "COMPLETED" && run.Status != "STOPPED" && run.Status != "FAILED" {
				fmt.Println("The workflow run hasn't been completed yet!")
				fmt.Println("Run ID: " + run.ID + "   Status: " + run.Status)
				continue
			}

			subJobs := getSubJobs(run.ID)
			for i := range subJobs {
				subJobs[i].Label = version.Data.Nodes[subJobs[i].NodeName].Meta.Label
			}

			labels := make([]LabelCnt, 0)
			for i := range subJobs {
				subJobs[i].Label = strings.ReplaceAll(subJobs[i].Label, "/", "-")
				found := false
				for _, l := range labels {
					if l.name == subJobs[i].Label {
						found = true
						break
					}
				}
				if !found {
					labels = append(labels, LabelCnt{name: subJobs[i].Label, cnt: 0})
				}
			}
			for _, sj := range subJobs {
				for j := range labels {
					if labels[j].name == sj.Label {
						labels[j].cnt++
						break
					}
				}
			}
			for j := range labels {
				if labels[j].cnt > 1 {
					labels[j].cnt++
				}
			}
			for i := len(subJobs) - 1; i > 0; i-- {
				for j := range labels {
					if labels[j].name == subJobs[i].Label && labels[j].cnt > 1 {
						subJobs[i].Label += "-" + strconv.Itoa(labels[j].cnt-1)
						labels[j].cnt--
						break
					}
				}
			}

			runDir := "run-" + run.StartedDate.Format(time.RFC3339)
			runDir = path.Join(args[0], runDir)
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
					getSubJobOutput(&subJob, true, "", runDir, 0)
				}
			} else {
				noneFound := true
				for _, subJob := range subJobs {
					_, labelExists := nodes[subJob.Label]
					if labelExists {
						nodes[subJob.Label] = NodeInfo{toFetch: true, found: true}
					}
					_, nameExists := nodes[subJob.Name]
					if nameExists {
						nodes[subJob.Name] = NodeInfo{toFetch: true, found: true}
					}
					_, nodeIDExists := nodes[subJob.NodeName]
					if nodeIDExists {
						nodes[subJob.NodeName] = NodeInfo{toFetch: true, found: true}
					}
					if nameExists || labelExists || nodeIDExists {
						noneFound = false
						getSubJobOutput(&subJob, true, "", runDir, 0)
					}
				}
				if noneFound {
					fmt.Println("Couldn't find any nodes that match given name(s)!")
				} else {
					for nodeName, nodeInfo := range nodes {
						if !nodeInfo.found {
							fmt.Println("Couldn't find any sub-job named " + nodeName + "!")
						}
					}
				}
			}
		}
	},
}

func init() {
	DownloadCmd.Flags().StringVar(&configFile, "config", "", "YAML file to determine which nodes output(s) should be downloaded")
	DownloadCmd.Flags().BoolVar(&allRuns, "all", false, "Download output data for all runs")
	DownloadCmd.Flags().IntVar(&numberOfRuns, "runs", 1, "Number of recent runs which outputs should be downloaded")
}

func getChildrenSubJobs(subJobID string) []types.SubJob {
	client := &http.Client{}

	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/subjob/"+subJobID+"/children/", nil)
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
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var subJobs types.SubJobs
	err = json.Unmarshal(bodyBytes, &subJobs)
	if err != nil {
		fmt.Println("Error unmarshalling sub-job children response!")
		return nil
	}

	return subJobs.Results
}

func getSubJobOutput(subJob *types.SubJob, fetchData bool, splitterDir string, runDir string, splitterTaskCount int) []types.SubJobOutput {
	client := &http.Client{}

	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/subjob-output/?subjob="+subJob.ID, nil)
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
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var subJobOutputs types.SubJobOutputs
	err = json.Unmarshal(bodyBytes, &subJobOutputs)
	if err != nil {
		fmt.Println("Error unmarshalling sub-job output response!")
		return nil
	}

	if subJob.TaskGroup {
		if splitterDir == "" {
			splitterDir = subJob.Label
		}
		splitterPath := strings.Split(splitterDir, "/")
		splitterDir = path.Join(runDir, splitterPath[len(splitterPath)-1])
		dirInfo, err := os.Stat(splitterDir)
		dirExists := !os.IsNotExist(err) && dirInfo.IsDir()

		if !dirExists {
			err = os.Mkdir(splitterDir, 0755)
			if err != nil {
				fmt.Println("Couldn't create a directory to store multiple outputs for " + subJob.Name + "!")
				os.Exit(0)
			}
		}

		children := getChildrenSubJobs(subJob.ID)
		if children == nil || len(children) == 0 {
			return nil
		}

		for i := range children {
			children[i].Label = strconv.Itoa(i) + "-" + subJob.Label
			getSubJobOutput(&children[i], true, splitterDir, runDir, subJob.TaskCount)
		}
	}

	dir := ""
	if len(subJobOutputs.Results) > 1 {
		dir = subJob.Label
		if subJob.TaskGroup && splitterTaskCount > 1 {
			dir = subJob.TaskIndex + "-" + dir
		}
		splitterPath := strings.Split(splitterDir, "/")
		dirPath := strings.Split(dir, "/")
		dir = path.Join(runDir, splitterPath[len(splitterPath)-1], dirPath[len(dirPath)-1])
		dirInfo, err := os.Stat(dir)
		dirExists := !os.IsNotExist(err) && dirInfo.IsDir()

		if !dirExists {
			err = os.Mkdir(dir, 0755)
			if err != nil {
				fmt.Println("Couldn't create a directory to store multiple outputs for " + subJob.Label + "!")
				os.Exit(0)
			}
		}
	}

	for i, output := range subJobOutputs.Results {
		req, err = http.NewRequest("POST", util.Cfg.BaseUrl+"v1/subjob-output/"+output.ID+"/signed_url/", nil)
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

				if dir == "" && splitterDir == "" {
					fileName = subJob.Label + "-" + fileName
				}
				if splitterDir != "" && splitterTaskCount > 1 {
					fileName = subJob.TaskIndex + "-" + fileName
				}
				splitterPath := strings.Split(splitterDir, "/")
				dirPath := strings.Split(dir, "/")
				fileName = path.Join(runDir, splitterPath[len(splitterPath)-1], dirPath[len(dirPath)-1], fileName)

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

				bar := progressbar.DefaultBytes(
					dataResp.ContentLength,
					"Downloading ["+subJob.Label+"] output... ",
				)
				_, err = io.Copy(io.MultiWriter(outputFile, bar), dataResp.Body)
				if err != nil {
					fmt.Println("Couldn't save data!")
					continue
				}

				_ = outputFile.Close()
				_ = dataResp.Body.Close()
				fmt.Println()
			}
		}
	}

	return subJobOutputs.Results
}

func getSubJobs(runID string) []types.SubJob {
	if runID == "" {
		fmt.Println("Couldn't list sub-jobs, no run ID parameter specified!")
		return nil
	}
	urlReq := util.Cfg.BaseUrl + "v1/subjob/?run=" + runID
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
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var subJobs types.SubJobs
	err = json.Unmarshal(bodyBytes, &subJobs)
	if err != nil {
		fmt.Println("Error unmarshalling sub-jobs response!")
		return nil
	}

	return subJobs.Results
}

func getRuns(workflowID string, pageSize int) []types.Run {
	urlReq := util.Cfg.BaseUrl + "v1/run/?vault=" + util.GetVault()

	if workflowID != "" {
		urlReq += "&workflow=" + workflowID
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
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var runs types.Runs
	err = json.Unmarshal(bodyBytes, &runs)
	if err != nil {
		fmt.Println("Error unmarshalling runs response!")
		return nil
	}

	return runs.Results
}

func getWorkflowVersionByID(id string) *types.WorkflowVersionDetailed {
	client := &http.Client{}

	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/store/workflow-version/"+id+"/", nil)
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
