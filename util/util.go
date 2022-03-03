package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"trickest-cli/types"
)

type UnexpectedResponse map[string]interface{}

const (
	BaseURL = "https://hive-api.beta.trickest.com/"
)

var Cfg = types.Config{
	BaseUrl: BaseURL,
}

func GetToken() string {
	if Cfg.User.Token == "" {
		tokenEnv, tokenSet := os.LookupEnv("TRICKEST_TOKEN")
		if tokenSet {
			Cfg.User.Token = tokenEnv
		} else {
			fmt.Println("Trickest authentication token not set! Use --token or TRICKEST_TOKEN environment variable.")
			os.Exit(0)
		}
	}
	return Cfg.User.Token
}

func GetVault() string {
	if Cfg.User.VaultId == "" {
		user := GetMe()
		if user != nil {
			Cfg.User.VaultId = user.Profile.VaultInfo.ID
		} else {
			fmt.Println("Couldn't get default vault ID! Check your Trickest token.")
			os.Exit(0)
		}
	}
	return Cfg.User.VaultId
}

func GetMe() *types.User {
	client := &http.Client{}

	req, err := http.NewRequest("GET", Cfg.BaseUrl+"v1/users/me/", nil)
	req.Header.Add("Authorization", "Token "+GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get user info. No http response!")
		os.Exit(0)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error: User with the specified token doesn't exist!")
		os.Exit(0)
	}

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read user info.")
		os.Exit(0)
	}

	var user types.User
	err = json.Unmarshal(bodyBytes, &user)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal user info.")
		os.Exit(0)
	}

	return &user
}

func GetHiveInfo() *types.Hive {
	client := &http.Client{}

	req, err := http.NewRequest("GET", Cfg.BaseUrl+"v1/hive/?vault="+GetVault()+"&demo=true", nil)
	req.Header.Add("Authorization", "Token "+GetToken())
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: Couldn't get hive ID.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Error: Couldn't read hive ID.")
		return nil
	}

	var hives types.Hives
	err = json.Unmarshal(bodyBytes, &hives)
	if err != nil {
		fmt.Println("Error unmarshalling hive response!")
		return nil
	}

	if hives.Results == nil && len(hives.Results) != 1 {
		fmt.Println("Couldn't obtain hive ID!")
		return nil
	}

	return &hives.Results[0]
}

func ProcessUnexpectedResponse(responseBody []byte, statusCode int) {
	if statusCode >= http.StatusInternalServerError {
		fmt.Println("Sorry, something went wrong!")
		os.Exit(0)
	}

	var response UnexpectedResponse
	err := json.Unmarshal(responseBody, &response)
	if err != nil {
		fmt.Println("Sorry, something went wrong!")
		os.Exit(0)
	}

	if details, exists := response["details"]; exists {
		fmt.Println(details)
		os.Exit(0)
	} else {
		fmt.Println("Sorry, something went wrong!")
		os.Exit(0)
	}
}
