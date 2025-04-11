package filesystem

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/schollz/progressbar/v3"
	"github.com/trickest/trickest-cli/pkg/trickest"
)

// CreateRunDir creates a directory for a run based on the run's started date
func CreateRunDir(baseDir string, run trickest.Run) (string, error) {
	if run.StartedDate == nil {
		return "", fmt.Errorf("run started date is nil, either the run has not started or the run object is incomplete or outdated")
	}

	const layout = "2006-01-02T150405Z"
	runDir := "run-" + run.StartedDate.Format(layout)
	runDir = path.Join(baseDir, runDir)

	if err := os.MkdirAll(runDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create run directory: %w", err)
	}

	return runDir, nil
}

// CreateSubJobDir creates a directory for a subjob based on the subjob's label
func CreateSubJobDir(runDir string, subJob trickest.SubJob) (string, error) {
	subJobDir := path.Join(runDir, subJob.Label)
	if err := os.MkdirAll(subJobDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create subjob directory: %w", err)
	}
	return subJobDir, nil
}

// DownloadFile downloads a file from a URL to the specified directory and shows a progress bar if showProgress is true
func DownloadFile(url, outputDir, fileName string, showProgress bool) error {
	// Create output directory if it doesn't exist, just in case
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	filePath := path.Join(outputDir, fileName)
	outputFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// Download the file
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	// Copy the content with or without progress bar
	if showProgress && resp.ContentLength > 0 {
		bar := progressbar.NewOptions64(
			resp.ContentLength,
			progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s... ", fileName)),
			progressbar.OptionSetWidth(30),
			progressbar.OptionShowBytes(true),
			progressbar.OptionShowCount(),
			progressbar.OptionOnCompletion(func() { fmt.Println() }),
		)
		_, err = io.Copy(io.MultiWriter(outputFile, bar), resp.Body)
	} else {
		_, err = io.Copy(outputFile, resp.Body)
	}

	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return nil
}
