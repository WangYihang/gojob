package pipeline_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/WangYihang/gojob/pipeline"
)

func TestWriteJSONL(t *testing.T) {
	ctx := context.Background()
	src := pipeline.From(ctx, "hello")
	results := pipeline.Process(ctx, src, func(ctx context.Context, s string) (string, error) {
		return strings.ToUpper(s), nil
	})
	var buf bytes.Buffer
	if err := pipeline.WriteJSONL(ctx, &buf, results); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", buf.String(), err)
	}
	if got["value"] != "HELLO" {
		t.Errorf("value: want HELLO, got %v", got["value"])
	}
	if got["error"] != "" {
		t.Errorf("error: want empty string, got %v", got["error"])
	}
}

func TestTee(t *testing.T) {
	ctx := context.Background()
	src := pipeline.From(ctx, 1, 2, 3)
	results := pipeline.Process(ctx, src, func(ctx context.Context, n int) (int, error) { return n, nil })
	outs := pipeline.Tee(ctx, results, 2)

	var wg sync.WaitGroup
	counts := make([]int, len(outs))
	for i, o := range outs {
		wg.Add(1)
		go func(i int, o <-chan pipeline.Result[int]) {
			defer wg.Done()
			for range o {
				counts[i]++
			}
		}(i, o)
	}
	wg.Wait()
	for i, c := range counts {
		if c != 3 {
			t.Errorf("tee branch %d: want 3 items, got %d", i, c)
		}
	}
}
