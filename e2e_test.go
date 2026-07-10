//go:build e2e

// End-to-end tests build an example binary and drive it as a real process
// against a local HTTP server. Run with: go test -tags=e2e ./...
package gojob_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestE2ECrawler(t *testing.T) {
	// Build the crawler example.
	bin := filepath.Join(t.TempDir(), "crawler")
	build := exec.Command("go", "build", "-o", bin, "./examples/crawler")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Local server returning 200 for everything.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Input file with a handful of URLs.
	input := filepath.Join(t.TempDir(), "urls.txt")
	var sb strings.Builder
	const n = 5
	for i := 0; i < n; i++ {
		sb.WriteString(srv.URL + "/p" + strconv.Itoa(i) + "\n")
	}
	if err := os.WriteFile(input, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run the binary, capturing JSONL on stdout. Bypass any sandbox proxy for
	// the local server.
	cmd := exec.Command(bin, "-i", input, "-o", "-", "-n", "4", "-r", "1", "-t", "5")
	cmd.Env = append(os.Environ(), "NO_PROXY=127.0.0.1,localhost", "no_proxy=127.0.0.1,localhost")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("crawler run failed: %v", err)
	}

	count := 0
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r struct {
			Value struct {
				URL        string `json:"url"`
				StatusCode int    `json:"status_code"`
			} `json:"value"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("invalid JSONL line %q: %v", line, err)
		}
		if r.Error != "" {
			t.Errorf("unexpected error for %s: %s", r.Value.URL, r.Error)
		}
		if r.Value.StatusCode != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d", r.Value.URL, r.Value.StatusCode)
		}
		count++
	}
	if count != n {
		t.Errorf("expected %d JSONL results, got %d", n, count)
	}
}
