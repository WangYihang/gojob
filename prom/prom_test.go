package prom_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/prom"
)

func TestPush(t *testing.T) {
	var pushes int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&pushes, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := context.Background()
	src := gojob.From(ctx, 1, 2, 3, 4, 5)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) { return n, nil })
	results, stats := gojob.WithStats(ctx, results, gojob.WithTotal(5))
	go gojob.Drain(results)

	if err := prom.Push(ctx, stats, srv.URL, "test", prom.WithInterval(1*time.Millisecond), prom.WithLabel("instance", "test-1")); err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if atomic.LoadInt32(&pushes) == 0 {
		t.Error("expected at least one push to the gateway")
	}
}
