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
	"time"
	"trickest-cli/cmd/list"
	"trickest-cli/types"
	"trickest-cli/util"
)

var configFile string

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

		_, _, workflow := list.ResolveObjectPath(args[0])
		if workflow == nil {
			return
		}

		run := getMostRecentRun(workflow.ID)
		if run == nil {
			return
		}

		if run.Status != "COMPLETED" {
			fmt.Println("The workflow run hasn't been completed yet!")
			fmt.Println("Status: " + run.Status)
			return
		}

		nodes := make(map[string]bool, 0)
		if len(args) > 1 {
			for i := 1; i < len(args); i++ {
				nodes[args[i]] = true
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
				nodes[node] = true
			}
		}

		subJobs := getSubJobs(run.ID)

		if len(nodes) == 0 {
			for _, subJob := range subJobs {
				getSubJobOutput(subJob.ID, true, "")
			}
		} else {
			for _, subJob := range subJobs {
				if nodes[subJob.Name] {
					getSubJobOutput(subJob.ID, true, "")
				}
			}
		}
	},
}

func init() {
	DownloadCmd.Flags().StringVar(&configFile, "config", "", "YAML file to determine which outputs should be downloaded")
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

func getSubJobOutput(subJobID string, fetchData bool, splitterDir string) []types.SubJobOutput {
	subJob := GetSubJobByID(subJobID)
	if subJob == nil {
		return nil
	}
	client := &http.Client{}

	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/subjob-output/?subjob="+subJobID, nil)
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

	nodeNameAndTimestamp := subJob.NodeName + " " + subJob.FinishedDate.Format(time.RFC1123)

	if subJob.TaskGroup {
		if splitterDir == "" {
			splitterDir = nodeNameAndTimestamp
		}
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

		for _, child := range children {
			getSubJobOutput(child.ID, true, splitterDir)
		}
	}

	dir := ""
	if len(subJobOutputs.Results) > 1 {
		dir = nodeNameAndTimestamp
		if subJob.TaskGroup {
			dir = subJob.TaskIndex + "-" + dir
		}
		dirInfo, err := os.Stat(dir)
		dirExists := !os.IsNotExist(err) && dirInfo.IsDir()

		if !dirExists {
			err = os.Mkdir(dir, 0755)
			if err != nil {
				fmt.Println("Couldn't create a directory to store multiple outputs for " + subJob.Name + "!")
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
				fileName := nodeNameAndTimestamp + " " + subJobOutputs.Results[i].FileName

				if splitterDir != "" {
					fileName = subJob.TaskIndex + "-" + fileName
				}

				fileName = path.Join(splitterDir, dir, fileName)

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

				bar := progressbar.DefaultBytes(
					dataResp.ContentLength,
					"Downloading ["+subJob.NodeName+"] output... ",
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

func GetSubJobByID(id string) *types.SubJob {
	client := &http.Client{}

	req, err := http.NewRequest("GET", util.Cfg.BaseUrl+"v1/subjob/"+id+"/", nil)
	req.Header.Add("Authorization", "Token "+util.Cfg.User.Token)
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get sub-job response.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read sub-job response.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		util.ProcessUnexpectedResponse(bodyBytes, resp.StatusCode)
	}

	var subJob types.SubJob
	err = json.Unmarshal(bodyBytes, &subJob)
	if err != nil {
		fmt.Println("Error unmarshalling sub-job response!")
		return nil
	}

	return &subJob
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

func getMostRecentRun(workflowID string) *types.Run {
	runs := getRuns(workflowID, 1)

	if runs == nil || len(runs) == 0 {
		fmt.Println("Couldn't find any run for the workflow: " + workflowID)
		return nil
	}

	return &runs[0]
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
