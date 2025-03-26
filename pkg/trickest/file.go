package trickest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
)

type File struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Size         int       `json:"size"`
	PrettySize   string    `json:"pretty_size"`
	ModifiedDate time.Time `json:"modified_date"`
}

// SearchFiles searches for files by query
func (c *Client) SearchFiles(ctx context.Context, query string) ([]File, error) {
	path := fmt.Sprintf("/file/?search=%s&vault=%s", query, c.vaultID)

	files, err := GetPaginated[File](c.Hive, ctx, path, 0)
	if err != nil {
		return nil, err
	}

	return files, nil
}

// GetFileByName retrieves a file by name by searching for it and returning the result with the exact name
func (c *Client) GetFileByName(ctx context.Context, name string) (File, error) {
	files, err := c.SearchFiles(ctx, name)
	if err != nil {
		return File{}, err
	}

	if len(files) == 0 {
		return File{}, fmt.Errorf("file not found: %s", name)
	}

	// loop through the results to find the file with the exact name
	for _, file := range files {
		if file.Name == name {
			return file, nil
		}
	}

	return File{}, fmt.Errorf("file not found: %s", name)
}

// GetFileSignedURL retrieves a signed URL for a file
func (c *Client) GetFileSignedURL(ctx context.Context, id uuid.UUID) (string, error) {
	path := fmt.Sprintf("/file/%s/signed_url/", id.String())

	var signedURL string
	if err := c.Hive.doJSON(ctx, http.MethodGet, path, nil, &signedURL); err != nil {
		return "", fmt.Errorf("failed to get file signed URL: %w", err)
	}

	return signedURL, nil
}

// DeleteFile deletes a file
func (c *Client) DeleteFile(ctx context.Context, id uuid.UUID) error {
	path := fmt.Sprintf("/file/%s/", id.String())

	resp, err := c.Hive.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// UploadFile uploads a file to the Trickest file storage
func (c *Client) UploadFile(ctx context.Context, filePath string, showProgress bool) (File, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return File{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileName := filepath.Base(filePath)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	defer writer.Close()

	part, err := writer.CreateFormFile("thumb", fileName)
	if err != nil {
		return File{}, fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy the content with or without progress bar
	if showProgress {
		stat, err := file.Stat()
		if err != nil {
			return File{}, fmt.Errorf("failed to get file stat: %w", err)
		}
		bar := progressbar.NewOptions64(
			stat.Size(),
			progressbar.OptionSetDescription(fmt.Sprintf("Uploading %s...", fileName)),
			progressbar.OptionSetWidth(30),
			progressbar.OptionShowBytes(true),
			progressbar.OptionShowCount(),
			progressbar.OptionOnCompletion(func() { fmt.Println() }),
		)
		_, err = io.Copy(io.MultiWriter(part, bar), file)
		if err != nil {
			return File{}, fmt.Errorf("failed to copy file: %w", err)
		}
	} else {
		_, err = io.Copy(part, file)
		if err != nil {
			return File{}, fmt.Errorf("failed to copy file: %w", err)
		}
	}

	_, err = part.Write([]byte("\n--" + writer.Boundary() + "--"))
	if err != nil {
		return File{}, fmt.Errorf("failed to complete form: %w", err)
	}

	path := "/file/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.Hive.basePath+path, body)
	if err != nil {
		return File{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return File{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return File{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var uploadedFile File
	if err := json.NewDecoder(resp.Body).Decode(&uploadedFile); err != nil {
		return File{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return uploadedFile, nil
}
