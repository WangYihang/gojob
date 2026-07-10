package gojob

import (
	"context"
	"encoding/json"
	"io"
)

// WriteJSONL encodes each result as one line of JSON to w until the stream ends
// or ctx is cancelled, returning the first encode error (or the ctx error).
func WriteJSONL[T any](ctx context.Context, w io.Writer, in <-chan Result[T]) error {
	enc := json.NewEncoder(w)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case r, ok := <-in:
			if !ok {
				return nil
			}
			if err := enc.Encode(r); err != nil {
				return err
			}
		}
	}
}

// Drain consumes and discards a stream. Useful for a tee'd branch that has no
// sink of its own.
func Drain[T any](in <-chan Result[T]) {
	for range in {
	}
}

// Tee duplicates the input into n independent streams; every result is
// delivered to all of them. Each output applies backpressure, so all of them
// must be consumed. The outputs close when the input does or ctx is cancelled.
func Tee[T any](ctx context.Context, in <-chan Result[T], n int) []<-chan Result[T] {
	outs := make([]chan Result[T], n)
	for i := range outs {
		outs[i] = make(chan Result[T])
	}
	go func() {
		defer func() {
			for _, o := range outs {
				close(o)
			}
		}()
		for r := range in {
			for _, o := range outs {
				select {
				case o <- r:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	ro := make([]<-chan Result[T], n)
	for i, o := range outs {
		ro[i] = o
	}
	return ro
}
