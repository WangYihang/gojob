package model

import (
	"context"
	"net/http"
	"time"
)

const MAX_TRIES = 4

type MyTask struct {
	Url  string `json:"url"`
	HTTP HTTP   `json:"http"`
}

func New(url string) *MyTask {
	return &MyTask{
		Url: url,
	}
}

func (t *MyTask) Do(_ context.Context) error {
	transport := &http.Transport{
		DisableCompression: true,
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 4 * time.Second,
	}
	req, err := http.NewRequest(http.MethodHead, t.Url, nil)
	if err != nil {
		return err
	}
	httpRequest, err := NewHTTPRequest(req)
	if err != nil {
		return err
	}
	t.HTTP.Request = httpRequest
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	httpResponse, err := NewHTTPResponse(resp)
	if err != nil {
		return err
	}
	t.HTTP.Response = httpResponse
	return nil
}
