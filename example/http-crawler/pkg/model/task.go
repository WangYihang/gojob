package model

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type MyTask struct {
	Url   string `json:"url"`
	HTTP  *HTTP  `json:"http"`
	Error string `json:"error"`
}

func NewTask(line []byte) *MyTask {
	t := &MyTask{
		Url:   "",
		HTTP:  &HTTP{},
		Error: "",
	}
	t.Unserialize(line)
	return t
}

func (t *MyTask) Unserialize(line []byte) (err error) {
	t.Url = string(bytes.TrimSpace(line))
	return
}

func (t *MyTask) Start() {
	client := &http.Client{
		// Disable follow redirection
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodHead, t.Url, nil)
	if err != nil {
		t.Error = err.Error()
		return
	}
	httpRequest, err := NewHTTPRequest(req)
	if err != nil {
		t.Error = err.Error()
		return
	}
	t.HTTP.Request = httpRequest
	resp, err := client.Do(req)
	if err != nil {
		t.Error = err.Error()
		return
	}
	httpResponse, err := NewHTTPResponse(resp)
	if err != nil {
		t.Error = err.Error()
		return
	}
	t.HTTP.Response = httpResponse
}

func (t *MyTask) Serialize() ([]byte, error) {
	return json.Marshal(t)
}
