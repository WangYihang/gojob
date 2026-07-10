package gojob_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WangYihang/gojob"
)

func TestWriteJSONL(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, "hello")
	results := gojob.Process(ctx, src, func(ctx context.Context, s string) (string, error) {
		return strings.ToUpper(s), nil
	})
	var buf bytes.Buffer
	if err := gojob.WriteJSONL(ctx, &buf, results); err != nil {
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

func TestWriteJSONLCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch := make(chan gojob.Result[int]) // never sends, never closes
	if err := gojob.WriteJSONL(ctx, io.Discard, ch); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestDrain(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, rangeInts(10)...)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) { return n, nil })
	// Drain should consume everything without panicking.
	gojob.Drain(results)
}

func TestTee(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, 1, 2, 3)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) { return n, nil })
	outs := gojob.Tee(ctx, results, 2)

	var wg sync.WaitGroup
	counts := make([]int, len(outs))
	for i, o := range outs {
		wg.Add(1)
		go func(i int, o <-chan gojob.Result[int]) {
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

func TestTeeCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	base := context.Background()
	src := gojob.From(base, rangeInts(1000)...)
	results := gojob.Process(base, src, func(ctx context.Context, n int) (int, error) { return n, nil })
	outs := gojob.Tee(ctx, results, 2)
	cancel()

	done := make(chan struct{})
	go func() {
		for _, o := range outs {
			for range o {
			}
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Tee outputs did not close after cancellation")
	}
}
