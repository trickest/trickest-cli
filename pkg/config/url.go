package config

import (
	"fmt"
	"net/url"

	"github.com/google/uuid"
)

func GetNodeIDFromWorkflowURL(workflowURL string) (string, error) {
	return geParameterValueFromURL(workflowURL, "node")
}

func GetRunIDFromWorkflowURL(workflowURL string) (uuid.UUID, error) {
	runID, err := geParameterValueFromURL(workflowURL, "run")
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(runID)
}

func geParameterValueFromURL(workflowURL string, parameter string) (string, error) {
	u, err := url.Parse(workflowURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL %q: %w", workflowURL, err)
	}

	queryParams, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", fmt.Errorf("failed to parse query parameters from URL %q: %w", workflowURL, err)
	}

	paramValues, found := queryParams[parameter]
	if !found {
		return "", fmt.Errorf("URL %q does not contain required parameter %q", workflowURL, parameter)
	}

	if len(paramValues) != 1 {
		return "", fmt.Errorf("URL %q contains %d values for parameter %q, expected exactly 1", workflowURL, len(paramValues), parameter)
	}

	return paramValues[0], nil
}
