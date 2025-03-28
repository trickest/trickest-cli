package util

import (
	"fmt"
	"log"
	"net/url"
	"os"
)

type Config struct {
	User struct {
		Token         string
		TokenFilePath string
	}
	BaseUrl    string
	Dependency string
}

var (
	Cfg          Config
	SpaceName    string
	ProjectName  string
	WorkflowName string
	URL          string
)

func GetToken() string {
	if Cfg.User.Token != "" {
		return Cfg.User.Token
	}

	if Cfg.User.TokenFilePath != "" {
		token, err := os.ReadFile(Cfg.User.TokenFilePath)
		if err != nil {
			log.Fatal("Couldn't read the token file: ", err)
		}
		Cfg.User.Token = string(token)
		return Cfg.User.Token
	}

	if tokenEnv, tokenSet := os.LookupEnv("TRICKEST_TOKEN"); tokenSet {
		Cfg.User.Token = tokenEnv
		return tokenEnv
	}

	log.Fatal("Trickest authentication token not set! Use --token, --token-file, or TRICKEST_TOKEN environment variable.")
	return ""
}

func GetNodeIDFromWorkflowURL(workflowURL string) (string, error) {
	return geParameterValueFromURL(workflowURL, "node")
}

func geParameterValueFromURL(workflowURL string, parameter string) (string, error) {
	u, err := url.Parse(workflowURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	queryParams, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", fmt.Errorf("invalid URL query: %w", err)
	}

	paramValues, found := queryParams[parameter]
	if !found {
		return "", fmt.Errorf("no %s parameter in the URL", parameter)
	}

	if len(paramValues) != 1 {
		return "", fmt.Errorf("invalid number of %s parameters in the URL: %d", parameter, len(paramValues))
	}

	return paramValues[0], nil
}
