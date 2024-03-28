package gojob

import (
	"encoding/json"
	"time"
)

type Status struct {
	Timestamp         string `json:"timestamp"`
	FailedTaskCount   int64  `json:"num_failed"`
	SucceedTaskCount  int64  `json:"num_succeed"`
	FinishedTaskCount int64  `json:"num_done"`
	TotalTaskCount    int64  `json:"num_total"`
}

func NewStatus(failedTaskCount, succeedTaskCount, totalTaskCount int64) *Status {
	return &Status{
		Timestamp:         time.Now().Format(time.RFC3339),
		FailedTaskCount:   failedTaskCount,
		SucceedTaskCount:  succeedTaskCount,
		FinishedTaskCount: failedTaskCount + succeedTaskCount,
		TotalTaskCount:    totalTaskCount,
	}
}

func (s Status) String() string {
	data, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(data)
}
