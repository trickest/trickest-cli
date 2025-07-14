package actions

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/trickest/trickest-cli/pkg/filesystem"
	"github.com/trickest/trickest-cli/pkg/trickest"
)

// DownloadResult represents the result of downloading outputs for a single subjob
type DownloadResult struct {
	SubJobName string
	FileName   string
	Success    bool
	Error      error
}

func PrintDownloadResults(results []DownloadResult, runID uuid.UUID, destinationPath string) {
	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failureCount++
			if result.FileName != "" {
				fmt.Fprintf(os.Stderr, "Warning: Failed to download file %q for node %q in run %s: %v\n", result.FileName, result.SubJobName, runID.String(), result.Error)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Failed to download output for node %q in run %s: %v\n", result.SubJobName, runID.String(), result.Error)
			}
		}
	}

	if failureCount > 0 {
		fmt.Fprintf(os.Stderr, "Download completed with %d successful and %d failed downloads for run %s into %q\n", successCount, failureCount, runID.String(), destinationPath+"/")
	} else if successCount > 0 {
		fmt.Printf("Successfully downloaded %d outputs from run %s into %q\n", successCount, runID.String(), destinationPath+"/")
	}
}

// DownloadRunOutput downloads the outputs for the specified nodes in the run
// Returns the download result summary, the directory where the outputs were saved, and an error if _all_ of the downloads failed
func DownloadRunOutput(client *trickest.Client, run *trickest.Run, nodes []string, files []string, destinationPath string) ([]DownloadResult, string, error) {
	if run.Status == "PENDING" || run.Status == "SUBMITTED" {
		return nil, "", fmt.Errorf("run %s has not started yet (status: %s)", run.ID.String(), run.Status)
	}

	ctx := context.Background()

	subJobs, err := client.GetSubJobs(ctx, *run.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get subjobs for run %s: %w", run.ID.String(), err)
	}

	// If the run was retrieved through the GetRuns() method, the WorkflowVersionInfo field will be nil
	if run.WorkflowVersionInfo == nil {
		run, err = client.GetRun(ctx, *run.ID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get run details for run %s: %w", run.ID.String(), err)
		}
	}
	version, err := client.GetWorkflowVersion(ctx, *run.WorkflowVersionInfo)
	if err != nil {
		return nil, "", fmt.Errorf("could not get workflow version for run %s: %w", run.ID.String(), err)
	}
	subJobs = trickest.LabelSubJobs(subJobs, *version)

	matchingSubJobs, unmatchedNodes := trickest.FilterSubJobs(subJobs, nodes)
	if len(matchingSubJobs) == 0 {
		return nil, "", fmt.Errorf("no completed node outputs matching your query %q were found in the run %s", strings.Join(nodes, ","), run.ID.String())
	}
	if len(unmatchedNodes) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: The following nodes were not found in run %s: %s. Proceeding with the remaining nodes\n", run.ID.String(), strings.Join(unmatchedNodes, ","))
	}

	runDir, err := filesystem.CreateRunDir(destinationPath, *run)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create directory for run %s: %w", run.ID.String(), err)
	}

	var allResults []DownloadResult
	var errCount int
	for _, subJob := range matchingSubJobs {
		isModule := version.Data.Nodes[subJob.Name].Type == "WORKFLOW"
		results := downloadSubJobOutput(client, runDir, &subJob, files, run.ID, isModule)

		for _, result := range results {
			if !result.Success {
				errCount++
			}
		}
		allResults = append(allResults, results...)
	}

	return allResults, runDir, nil
}

func downloadSubJobOutput(client *trickest.Client, savePath string, subJob *trickest.SubJob, files []string, runID *uuid.UUID, isModule bool) []DownloadResult {
	if !subJob.TaskGroup && subJob.Status != "SUCCEEDED" {
		return []DownloadResult{{
			SubJobName: subJob.Label,
			FileName:   "",
			Success:    false,
			Error:      fmt.Errorf("subjob %s (ID: %s) is not completed (status: %s)", subJob.Label, subJob.ID, subJob.Status),
		}}
	}

	if subJob.TaskGroup {
		return downloadTaskGroupOutput(client, savePath, subJob, files, runID)
	}

	return downloadSingleSubJobOutput(client, savePath, subJob, files, runID, isModule)
}

func downloadTaskGroupOutput(client *trickest.Client, savePath string, subJob *trickest.SubJob, files []string, runID *uuid.UUID) []DownloadResult {
	ctx := context.Background()
	children, err := client.GetChildSubJobs(ctx, subJob.ID)
	if err != nil {
		return []DownloadResult{{
			SubJobName: subJob.Label,
			FileName:   "",
			Success:    false,
			Error:      fmt.Errorf("could not get child subjobs for subjob %s (ID: %s): %w", subJob.Label, subJob.ID, err),
		}}
	}
	if len(children) == 0 {
		return []DownloadResult{{
			SubJobName: subJob.Label,
			FileName:   "",
			Success:    false,
			Error:      fmt.Errorf("no child subjobs found for subjob %s (ID: %s)", subJob.Label, subJob.ID),
		}}
	}

	var mu sync.Mutex
	var results []DownloadResult
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for i := 1; i <= len(children); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			child, err := client.GetChildSubJob(ctx, subJob.ID, i)
			if err != nil {
				mu.Lock()
				results = append(results, DownloadResult{
					SubJobName: fmt.Sprintf("%d-%s", i, subJob.Label),
					FileName:   "",
					Success:    false,
					Error:      fmt.Errorf("could not get child %d subjobs for subjob %s (ID: %s): %w", i, subJob.Label, subJob.ID, err),
				})
				mu.Unlock()
				return
			}

			child.Label = fmt.Sprintf("%d-%s", i, subJob.Label)
			childResults := downloadSubJobOutput(client, savePath, &child, files, runID, false)

			mu.Lock()
			results = append(results, childResults...)
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	return results
}

func downloadSingleSubJobOutput(client *trickest.Client, savePath string, subJob *trickest.SubJob, files []string, runID *uuid.UUID, isModule bool) []DownloadResult {
	ctx := context.Background()

	subJobOutputs, err := getSubJobOutputs(client, ctx, subJob, runID, isModule)
	if err != nil {
		return []DownloadResult{{
			SubJobName: subJob.Label,
			FileName:   "",
			Success:    false,
			Error:      err,
		}}
	}

	subJobOutputs = filterSubJobOutputsByFileNames(subJobOutputs, files)
	if len(subJobOutputs) == 0 {
		return []DownloadResult{{
			SubJobName: subJob.Label,
			FileName:   "",
			Success:    false,
			Error:      fmt.Errorf("no matching output files found for subjob %s (ID: %s)", subJob.Label, subJob.ID),
		}}
	}

	var results []DownloadResult

	for _, output := range subJobOutputs {
		if err := downloadOutput(client, savePath, subJob, output); err != nil {
			results = append(results, DownloadResult{
				SubJobName: subJob.Label,
				FileName:   output.Name,
				Success:    false,
				Error:      err,
			})
		} else {
			results = append(results, DownloadResult{
				SubJobName: subJob.Label,
				FileName:   output.Name,
				Success:    true,
				Error:      nil,
			})
		}
	}

	return results
}

func getSubJobOutputs(client *trickest.Client, ctx context.Context, subJob *trickest.SubJob, runID *uuid.UUID, isModule bool) ([]trickest.SubJobOutput, error) {
	if isModule {
		outputs, err := client.GetModuleSubJobOutputs(ctx, subJob.Name, *runID)
		if err != nil {
			return nil, fmt.Errorf("could not get subjob outputs for subjob %s (ID: %s): %w", subJob.Label, subJob.ID, err)
		}
		return outputs, nil
	}

	outputs, err := client.GetSubJobOutputs(ctx, subJob.ID)
	if err != nil {
		return nil, fmt.Errorf("could not get subjob outputs for subjob %s (ID: %s): %w", subJob.Label, subJob.ID, err)
	}
	return outputs, nil
}

func downloadOutput(client *trickest.Client, savePath string, subJob *trickest.SubJob, output trickest.SubJobOutput) error {
	signedURL, err := client.GetOutputSignedURL(context.Background(), output.ID)
	if err != nil {
		return fmt.Errorf("could not get signed URL for output %s of subjob %s (ID: %s): %w", output.Name, subJob.Label, subJob.ID, err)
	}

	subJobDir, err := filesystem.CreateSubJobDir(savePath, *subJob)
	if err != nil {
		return fmt.Errorf("could not create directory to store output %s: %w", output.Name, err)
	}

	if err := filesystem.DownloadFile(signedURL.Url, subJobDir, output.Name, true); err != nil {
		return fmt.Errorf("could not download file for output %s of subjob %s (ID: %s): %w", output.Name, subJob.Label, subJob.ID, err)
	}

	return nil
}

func filterSubJobOutputsByFileNames(outputs []trickest.SubJobOutput, fileNames []string) []trickest.SubJobOutput {
	if len(fileNames) == 0 {
		return outputs
	}

	var matchingOutputs []trickest.SubJobOutput
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
