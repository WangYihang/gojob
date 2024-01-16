package model

import (
	"encoding/json"
	"net/http"
	"time"
)

type MyTask struct {
	Url        string `json:"url"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	HTTP       HTTP   `json:"http"`
	Error      string `json:"error"`
}

func New(url string) *MyTask {
	return &MyTask{
		Url: url,
	}
}

func (t *MyTask) Do() error {
	t.StartedAt = time.Now().UnixMilli()
	defer func() {
		t.FinishedAt = time.Now().UnixMilli()
	}()

	client := &http.Client{
		// Disable follow redirection
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodHead, t.Url, nil)
	if err != nil {
		t.Error = err.Error()
		return err
	}
	httpRequest, err := NewHTTPRequest(req)
	if err != nil {
		t.Error = err.Error()
		return err
	}
	t.HTTP.Request = httpRequest
	resp, err := client.Do(req)
	if err != nil {
		t.Error = err.Error()
		return err
	}
	httpResponse, err := NewHTTPResponse(resp)
	if err != nil {
		t.Error = err.Error()
		return err
	}
	t.HTTP.Response = httpResponse
	return nil
}

func (t *MyTask) Bytes() ([]byte, error) {
	return json.Marshal(t)
}

func (t *MyTask) NeedRetry() bool {
	return false
}
