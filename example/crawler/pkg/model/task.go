package model

import (
	"net/http"
	"net/url"
)

type Task struct {
	Url        *url.URL `json:"url"`
	StartedAt  int64    `json:"started_at"`
	FinishedAt int64    `json:"finished_at"`
	Error      string   `json:"error"`
}

func NewTask(rawUrl string) (*Task, error) {
	u, err := url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}
	return &Task{
		Url: u,
	}, nil
}

func (t *Task) String() string {
	return t.Url.String()
}

func (t *Task) Run() (result *Result) {
	var err error
	result = &Result{
		Task:  t,
		HTTP:  &HTTP{},
		Error: "",
	}
	client := &http.Client{
		// Disable follow redirection
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodHead, t.Url.String(), nil)
	if err != nil {
		result.Error = err.Error()
		return
	}
	httpRequest, err := NewHTTPRequest(req)
	if err != nil {
		result.Error = err.Error()
		return
	}
	result.HTTP.Request = httpRequest
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return
	}
	httpResponse, err := NewHTTPResponse(resp)
	if err != nil {
		result.Error = err.Error()
		return
	}
	result.HTTP.Response = httpResponse
	return
}
