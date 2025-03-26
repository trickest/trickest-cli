package actions

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/trickest/trickest-cli/pkg/filesystem"
	"github.com/trickest/trickest-cli/pkg/trickest"
)

func DownloadRunOutput(client *trickest.Client, run *trickest.Run, nodes []string, files []string, destinationPath string) error {
	if run.Status == "PENDING" || run.Status == "SUBMITTED" {
		return fmt.Errorf("run %s has not started yet (status: %s)", run.ID.String(), run.Status)
	}

	ctx := context.Background()

	subJobs, err := client.GetSubJobs(ctx, *run.ID)
	if err != nil {
		return fmt.Errorf("failed to get subjobs for run %s: %w", run.ID.String(), err)
	}

	version, err := client.GetWorkflowVersion(ctx, *run.WorkflowVersionInfo)
	if err != nil {
		return fmt.Errorf("could not get workflow version for run %s: %w", run.ID.String(), err)
	}
	subJobs = trickest.LabelSubJobs(subJobs, *version)

	matchingSubJobs, err := filterSubJobs(subJobs, nodes)
	if err != nil {
		return fmt.Errorf("no completed node outputs matching your query were found in the run %s: %w", run.ID.String(), err)
	}

	runDir, err := filesystem.CreateRunDir(destinationPath, *run)
	if err != nil {
		return fmt.Errorf("failed to create directory for run %s: %w", run.ID.String(), err)
	}

	for _, subJob := range matchingSubJobs {
		isModule := version.Data.Nodes[subJob.Name].Type == "WORKFLOW"
		if err := downloadSubJobOutput(client, runDir, &subJob, files, run.ID, isModule); err != nil {
			return fmt.Errorf("error downloading output for node %s: %w", subJob.Label, err)
		}
	}

	return nil
}

func downloadSubJobOutput(client *trickest.Client, savePath string, subJob *trickest.SubJob, files []string, runID *uuid.UUID, isModule bool) error {
	if !subJob.TaskGroup && subJob.Status != "SUCCEEDED" {
		return fmt.Errorf("subjob %s (ID: %s) is not completed (status: %s)", subJob.Label, subJob.ID, subJob.Status)
	}

	if subJob.TaskGroup {
		return downloadTaskGroupOutput(client, savePath, subJob, files, runID)
	}

	return downloadSingleSubJobOutput(client, savePath, subJob, files, runID, isModule)
}

func downloadTaskGroupOutput(client *trickest.Client, savePath string, subJob *trickest.SubJob, files []string, runID *uuid.UUID) error {
	ctx := context.Background()
	children, err := client.GetChildSubJobs(ctx, subJob.ID)
	if err != nil {
		return fmt.Errorf("could not get child subjobs for subjob %s (ID: %s): %w", subJob.Label, subJob.ID, err)
	}
	if len(children) == 0 {
		return fmt.Errorf("no child subjobs found for subjob %s (ID: %s)", subJob.Label, subJob.ID)
	}

	var mu sync.Mutex
	var errs []error
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
				errs = append(errs, fmt.Errorf("could not get child %d subjobs for subjob %s (ID: %s): %w", i, subJob.Label, subJob.ID, err))
				mu.Unlock()
				return
			}

			child.Label = fmt.Sprintf("%d-%s", i, subJob.Label)
			if err := downloadSubJobOutput(client, savePath, &child, files, runID, false); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred while downloading subjob children outputs:\n%s", errors.Join(errs...))
	}
	return nil
}

func downloadSingleSubJobOutput(client *trickest.Client, savePath string, subJob *trickest.SubJob, files []string, runID *uuid.UUID, isModule bool) error {
	ctx := context.Background()
	var errs []error

	subJobOutputs, err := getSubJobOutputs(client, ctx, subJob, runID, isModule)
	if err != nil {
		return err
	}

	subJobOutputs = filterSubJobOutputsByFileNames(subJobOutputs, files)
	if len(subJobOutputs) == 0 {
		return fmt.Errorf("no matching output files found for subjob %s (ID: %s)", subJob.Label, subJob.ID)
	}

	for _, output := range subJobOutputs {
		if err := downloadOutput(client, savePath, subJob, output); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred while downloading subjob outputs:\n%s", errors.Join(errs...))
	}
	return nil
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

func filterSubJobs(subJobs []trickest.SubJob, identifiers []string) ([]trickest.SubJob, error) {
	if len(identifiers) == 0 {
		return subJobs, nil
	}

	var foundNodes []string
	var matchingSubJobs []trickest.SubJob

	for _, subJob := range subJobs {
		labelExists := slices.Contains(identifiers, subJob.Label)
		nameExists := slices.Contains(identifiers, subJob.Name)

		if labelExists {
			foundNodes = append(foundNodes, subJob.Label)
		}
		if nameExists {
			foundNodes = append(foundNodes, subJob.Name)
		}

		if labelExists || nameExists {
			matchingSubJobs = append(matchingSubJobs, subJob)
		}
	}

	for _, identifier := range identifiers {
		if !slices.Contains(foundNodes, identifier) {
			return nil, fmt.Errorf("subjob with name or label %s not found", identifier)
		}
	}

	return matchingSubJobs, nil
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
