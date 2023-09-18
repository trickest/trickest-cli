package request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type Request struct {
	client  *http.Client
	timeout time.Duration

	headers  http.Header
	version  string
	endpoint string

	verb string
	body io.Reader

	err error

	response *Response
}

type Response struct {
	err      error
	response *http.Response
	body     []byte
}

var Trickest *Request

func New() *Request {
	r := new(Request)
	r.response = new(Response)
	r.headers = make(map[string][]string)
	r.Header("Content-Type", "application/json")
	r.client = new(http.Client)
	r.body = nil
	return r
}

func (r *Request) Version(version string) *Request {
	r.version = version
	return r
}

func (r *Request) Endpoint(endpoint string) *Request {
	r.endpoint = strings.Trim(endpoint, "/")
	return r
}

func (r *Request) Header(header string, value string) *Request {
	if _, ok := r.headers[header]; !ok {
		r.headers[header] = []string{}
	} else {
		if header == "Authorization" {
			return r
		}
	}
	r.headers[header] = append(r.headers[header], value)
	return r
}

func (r *Request) Get() *Request {
	return r.Verb("GET")
}

func (r *Request) Post() *Request {
	return r.Verb("POST")
}

func (r *Request) Put() *Request {
	return r.Verb("PUT")
}

func (r *Request) Patch() *Request {
	return r.Verb("PATCH")
}

func (r *Request) Delete() *Request {
	return r.Verb("DELETE")
}

func (r *Request) Body(body []byte) *Request {
	r.body = bytes.NewReader(body)
	return r
}

func (r *Request) Verb(verb string) *Request {
	r.verb = verb
	return r
}

func (r *Request) DoF(url string, v ...interface{}) *Response {
	if len(v) > 0 {
		url = fmt.Sprintf(url, v...)
	}
	return r.Do(url)
}

func (r *Request) Do(url string) *Response {
	req, err := r.newRequest(url)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	r.client.Timeout = r.timeout
	resp, err := r.client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer resp.Body.Close()

	r.response.body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	r.response.response = resp

	return r.response
}

func (r *Response) Status() int {
	return r.response.StatusCode
}

func (r *Response) Body() []byte {
	return r.body
}

func (r *Request) newRequest(url string) (*http.Request, error) {
	_url := fmt.Sprintf("%s/%s/%s", r.endpoint, r.version, url)
	req, err := http.NewRequest(r.verb, _url, r.body)
	if err != nil {
		return nil, err
	}
	req.Header = r.headers
	return req, nil
}

func ProcessUnexpectedResponse(resp *Response) {
	if resp == nil || resp.response == nil {
		fmt.Println("Response is nil")
		os.Exit(0)
	}

	//fmt.Println(resp.response.Request.Method + " " + resp.response.Request.URL.Path + " " + strconv.Itoa(resp.response.StatusCode))
	//if len(resp.body) > 0 {
	//	fmt.Println("Response body:\n" + string(resp.body))
	//}

	if resp.response.StatusCode >= http.StatusInternalServerError {
		fmt.Println("Sorry, something went wrong!")
		os.Exit(0)
	}

	if resp.response.StatusCode == http.StatusUnauthorized {
		fmt.Println("Error: Unauthorized to perform this action.\nPlease, make sure that your token is correct and that you have access to this resource.")
		os.Exit(0)
	}

	var response map[string]interface{}
	err := json.Unmarshal(resp.body, &response)
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
