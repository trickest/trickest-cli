package trickest

import (
	"context"
	"fmt"
	"net/http"
)

const defaultPageSize = 500

// PaginatedResponse represents a generic paginated response from the API
type PaginatedResponse[T any] struct {
	Count    int    `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Results  []T    `json:"results"`
}

// withPagination adds pagination parameters to a URL path
func withPagination(path string, page int) string {
	path += fmt.Sprintf("&page_size=%d", defaultPageSize)
	if page > 0 {
		path += fmt.Sprintf("&page=%d", page)
	}
	return path
}

// GetPaginated gets results from a paginated endpoint with a limit.
// If limit is 0, it will get all results.
func GetPaginated[T any](c *Client, ctx context.Context, path string, limit int) ([]T, error) {
	var allResults []T
	currentPage := 1

	for {
		var response PaginatedResponse[T]
		currentPath := withPagination(path, currentPage)
		if err := c.doJSON(ctx, http.MethodGet, currentPath, nil, &response); err != nil {
			return nil, fmt.Errorf("failed to get page %d: %w", currentPage, err)
		}

		allResults = append(allResults, response.Results...)

		if limit > 0 && len(allResults) >= limit {
			allResults = allResults[:limit]
			break
		}

		if response.Next == "" {
			break
		}

		currentPage++
	}

	return allResults, nil
}
