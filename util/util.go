package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"trickest-cli/types"
)

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
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read user info.")
		return nil
	}

	var user types.User
	err = json.Unmarshal(bodyBytes, &user)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal user info.")
		return nil
	}

	return &user
}
