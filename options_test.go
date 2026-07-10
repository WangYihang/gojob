package gojob_test

import (
	"testing"
	"time"

	"github.com/WangYihang/gojob"
)

func TestExpBackoff(t *testing.T) {
	b := gojob.ExpBackoff(100*time.Millisecond, 1*time.Second)
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1 * time.Second}, // 1600ms capped to 1s
		{6, 1 * time.Second}, // capped
	}
	for _, c := range cases {
		if got := b(c.attempt); got != c.want {
			t.Errorf("ExpBackoff(attempt=%d): want %v, got %v", c.attempt, c.want, got)
		}
	}
}

func TestNoBackoff(t *testing.T) {
	if d := gojob.NoBackoff()(3); d != 0 {
		t.Errorf("NoBackoff: want 0, got %v", d)
	}
}
