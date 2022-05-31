package util

import (
	"encoding/json"
	"fmt"
	"github.com/hako/durafmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"trickest-cli/types"
)

type UnexpectedResponse map[string]interface{}

const (
	BaseURL = "https://hive-api.beta.trickest.com/"
)

var (
	Cfg = types.Config{
		BaseUrl: BaseURL,
	}
	SpaceName    string
	ProjectName  string
	WorkflowName string
)

func FormatPath() string {
	path := strings.Trim(SpaceName, "/")
	if ProjectName != "" {
		path += "/" + strings.Trim(ProjectName, "/")
	}
	if WorkflowName != "" {
		path += "/" + strings.Trim(WorkflowName, "/")
	}
	return path
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

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read user info.")
		os.Exit(0)
	}

	if resp.StatusCode != http.StatusOK {
		ProcessUnexpectedResponse(resp)
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
		fmt.Println("Error: Couldn't get hive info.")
		return nil
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read hive info.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		ProcessUnexpectedResponse(resp)
	}

	var hives types.Hives
	err = json.Unmarshal(bodyBytes, &hives)
	if err != nil {
		fmt.Println("Error unmarshalling hive response!")
		return nil
	}

	if hives.Results == nil || len(hives.Results) == 0 {
		fmt.Println("Couldn't find hive info!")
		return nil
	}

	return &hives.Results[0]
}

func ProcessUnexpectedResponse(resp *http.Response) {
	fmt.Println(resp.Request.Method + " " + resp.Request.URL.Path + " " + strconv.Itoa(resp.StatusCode))
	if resp.StatusCode >= http.StatusInternalServerError {
		fmt.Println("Sorry, something went wrong!")
		os.Exit(0)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: Couldn't read unexpected response.")
		os.Exit(0)
	}

	var response UnexpectedResponse
	err = json.Unmarshal(bodyBytes, &response)
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

func FormatDuration(duration time.Duration) string {
	duration = duration.Round(time.Second)
	units := durafmt.Units{
		Year:   durafmt.Unit{Singular: "year", Plural: "years"},
		Week:   durafmt.Unit{Singular: "week", Plural: "weeks"},
		Day:    durafmt.Unit{Singular: "day", Plural: "days"},
		Hour:   durafmt.Unit{Singular: "h", Plural: "h"},
		Minute: durafmt.Unit{Singular: "m", Plural: "m"},
		Second: durafmt.Unit{Singular: "s", Plural: "s"},
	}

	str := durafmt.Parse(duration).LimitFirstN(2).Format(units)
	str = strings.Replace(str, " s", "s", 1)
	str = strings.Replace(str, " m", "m", 1)
	str = strings.Replace(str, " h", "h", 1)

	return str
}
