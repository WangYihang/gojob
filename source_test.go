package gojob_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/WangYihang/gojob"
)

func TestFrom(t *testing.T) {
	ctx := context.Background()
	var got []int
	for v := range gojob.From(ctx, 1, 2, 3) {
		got = append(got, v)
	}
	if len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Errorf("From: want [1 2 3], got %v", got)
	}
}

func TestFromCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	count := 0
	for range gojob.From(ctx, rangeInts(1000)...) {
		count++
	}
	if count >= 1000 {
		t.Errorf("expected cancellation to cut the stream short, got %d", count)
	}
}

func TestLinesFile(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("  a  \nb\n\n  c \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got []string
	for line := range gojob.Lines(ctx, path) {
		got = append(got, line)
	}
	// Lines trims whitespace but does not drop blank lines.
	want := []string{"a", "b", "", "c"}
	if len(got) != len(want) {
		t.Fatalf("Lines: want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Lines[%d]: want %q, got %q", i, want[i], got[i])
		}
	}
}

func TestLinesMissing(t *testing.T) {
	ctx := context.Background()
	count := 0
	for range gojob.Lines(ctx, "/nonexistent-dir-gojob/missing.txt") {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 lines from an unopenable source, got %d", count)
	}
}
