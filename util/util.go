package util

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"os"
	"strings"
	"time"
	"trickest-cli/client/request"
	"trickest-cli/types"

	"github.com/hako/durafmt"
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

func CreateRequest() {
	request.Trickest = request.New().Endpoint(Cfg.BaseUrl).Version("v1").Header("Authorization", "Token "+GetToken())
}

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

func GetVault() uuid.UUID {
	if Cfg.User.VaultId == uuid.Nil {
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
	resp := request.Trickest.Get().DoF("users/me/")
	if resp == nil || resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var user types.User
	err := json.Unmarshal(resp.Body(), &user)
	if err != nil {
		fmt.Println("Error: Couldn't unmarshal user info.")
		os.Exit(0)
	}

	return &user
}

func GetHiveInfo() *types.Hive {
	resp := request.Trickest.Get().DoF("hive/?vault=%s&demo=true", GetVault())
	if resp == nil || resp.Status() != http.StatusOK {
		request.ProcessUnexpectedResponse(resp)
	}

	var hives types.Hives
	err := json.Unmarshal(resp.Body(), &hives)
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
