package model

type Result struct {
	Task  *Task  `json:"task"`
	HTTP  *HTTP  `json:"http"`
	Error string `json:"error"`
}
