package runner

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

var Runner *IPInfo
var once sync.Once

func init() {
	once.Do(func() {
		Runner = NewIPInfo()
		err := Runner.Get()
		if err != nil {
			slog.Error("error occured while getting runner ip info", slog.String("error", err.Error()))
			return
		}
	})
}

type IPInfo struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
}

func NewIPInfo() *IPInfo {
	return &IPInfo{
		IP:       "unknown",
		Hostname: "unknown",
		City:     "unknown",
		Region:   "unknown",
		Country:  "unknown",
		Loc:      "unknown",
		Org:      "unknown",
		Postal:   "unknown",
		Timezone: "unknown",
	}
}

func (i *IPInfo) Get() error {
	req, err := http.Get("https://ipinfo.io/")
	if err != nil {
		return err
	}
	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, i)
	if err != nil {
		return err
	}
	return nil
}
