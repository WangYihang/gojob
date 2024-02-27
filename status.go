package gojob

import "encoding/json"

type Status struct {
	Timestamp string `json:"timestamp"`
	NumDone   int64  `json:"num_done"`
	NumTotal  int64  `json:"num_total"`
}

func (s Status) String() string {
	data, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(data)
}
