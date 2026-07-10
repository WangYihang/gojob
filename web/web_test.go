package web_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/web"
)

func noProxy() *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: nil}}
}

func TestDashboardIndex(t *testing.T) {
	ctx := context.Background()
	_, stats := gojob.WithStats(ctx, gojob.Process(ctx, gojob.From(ctx, 1), func(ctx context.Context, n int) (int, error) { return n, nil }))

	ts := httptest.NewServer(web.Handler(ctx, stats, web.WithTitle("my job")))
	defer ts.Close()

	resp, err := noProxy().Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("index status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("index content-type = %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "my job") {
		t.Error("index page should contain the configured title")
	}
	if !strings.Contains(string(body), "EventSource('/events')") {
		t.Error("index page should wire up the SSE stream")
	}
}

func TestDashboardEvents(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, 1, 2, 3)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		if n == 2 {
			return 0, io.EOF
		}
		return n, nil
	})
	results, stats := gojob.WithStats(ctx, results, gojob.WithTotal(3))
	gojob.Drain(results) // run the job to completion first

	ts := httptest.NewServer(web.Handler(ctx, stats, web.WithInterval(10*time.Millisecond)))
	defer ts.Close()

	// Because the job is already finished, /events sends the current snapshot,
	// then a final one, then closes — so reading the whole body terminates.
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/events", nil)
	resp, err := noProxy().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("events content-type = %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)

	var last map[string]any
	sawFinished := false
	for _, line := range strings.Split(string(body), "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(data), &last); err != nil {
			t.Fatalf("invalid SSE payload %q: %v", data, err)
		}
		if last["finished"] == true {
			sawFinished = true
		}
	}
	if last == nil {
		t.Fatal("expected at least one SSE data event")
	}
	if last["done"].(float64) != 3 || last["succeeded"].(float64) != 2 || last["failed"].(float64) != 1 {
		t.Errorf("unexpected final snapshot: %+v", last)
	}
	if !sawFinished {
		t.Error("expected a finished=true event")
	}
}

func TestServeShutsDown(t *testing.T) {
	// Grab a free port, then let Serve bind it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	_, stats := gojob.WithStats(ctx, gojob.Process(ctx, gojob.From(ctx, 1), func(ctx context.Context, n int) (int, error) { return n, nil }))

	errCh := make(chan error, 1)
	go func() { errCh <- web.Serve(ctx, stats, addr) }()

	client := noProxy()
	up := false
	for i := 0; i < 100; i++ {
		resp, err := client.Get("http://" + addr + "/")
		if err == nil {
			resp.Body.Close()
			up = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !up {
		cancel()
		t.Fatal("server did not come up")
	}

	cancel() // trigger graceful shutdown
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Serve returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not shut down after ctx cancellation")
	}
}
