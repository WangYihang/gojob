//go:build integration

// Integration tests exercise the whole pipeline against a real (in-process)
// HTTP server. Run with: go test -tags=integration ./...
package gojob_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/WangYihang/gojob"
)

// noProxyClient avoids any ambient HTTP(S)_PROXY so requests to the local test
// server are not routed through a sandbox proxy.
func noProxyClient() *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: nil}}
}

func get(client *http.Client) func(context.Context, string) (int, error) {
	return func(ctx context.Context, url string) (int, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return 0, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			return resp.StatusCode, fmt.Errorf("server error %d", resp.StatusCode)
		}
		return resp.StatusCode, nil
	}
}

func TestIntegrationCrawlPipeline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := context.Background()
	var urls []string
	for i := 0; i < 50; i++ {
		urls = append(urls, srv.URL+"/ok?i="+strconv.Itoa(i))
	}
	urls = append(urls, srv.URL+"/fail")

	results := gojob.Process(ctx, gojob.From(ctx, urls...), get(noProxyClient()),
		gojob.WithWorkers(8),
		gojob.WithRetry(2, gojob.NoBackoff()),
		gojob.WithTimeout(5*time.Second),
	)
	results, stats := gojob.WithStats(ctx, results, gojob.WithTotal(int64(len(urls))))

	ok, failed := 0, 0
	for r := range results {
		if r.Err != nil {
			failed++
		} else {
			ok++
		}
	}
	if ok != 50 {
		t.Errorf("expected 50 successes, got %d", ok)
	}
	if failed != 1 {
		t.Errorf("expected 1 failure, got %d", failed)
	}
	if snap := stats.Snapshot(); snap.Done != 51 || snap.Succeeded != 50 || snap.Failed != 1 {
		t.Errorf("unexpected stats: %+v", snap)
	}
}

func TestIntegrationRetryRecovers(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.URL.Path]++
		n := seen[r.URL.Path]
		mu.Unlock()
		if n < 2 { // fail the first attempt, succeed on the retry
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := context.Background()
	var urls []string
	for i := 0; i < 10; i++ {
		urls = append(urls, srv.URL+"/p"+strconv.Itoa(i))
	}

	results := gojob.Process(ctx, gojob.From(ctx, urls...), get(noProxyClient()),
		gojob.WithWorkers(4),
		gojob.WithRetry(3, gojob.NoBackoff()),
		gojob.WithTimeout(5*time.Second),
	)

	for r := range results {
		if r.Err != nil {
			t.Errorf("expected recovery on retry, got error: %v", r.Err)
		}
		if r.Attempts != 2 {
			t.Errorf("expected success on attempt 2, got %d", r.Attempts)
		}
	}
}
