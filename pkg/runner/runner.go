package runner

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

var (
	runner *IPInfo
	once   sync.Once
)

// Get returns information about the current runner (its public IP, geo
// location, etc.).
//
// The information is fetched lazily from ipinfo.io on the first call and cached
// for subsequent calls. Fetching only happens when Get is invoked (for example
// when the Prometheus push gateway is enabled), so simply importing this
// package no longer triggers any network request. If the lookup fails, an
// IPInfo populated with "unknown" placeholder values is returned.
func Get() *IPInfo {
	once.Do(func() {
		runner = NewIPInfo()
		if err := runner.Get(); err != nil {
			slog.Warn("error occurred while getting runner ip info", slog.String("error", err.Error()))
		}
	})
	return runner
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
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://ipinfo.io/")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, i)
}
