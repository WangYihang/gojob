package gojob

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/WangYihang/uio"
)

// From streams the given items in order and then closes.
func From[T any](ctx context.Context, items ...T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for _, it := range items {
			select {
			case out <- it:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// Lines streams the whitespace-trimmed lines of path, which may be a local
// file, "-" for stdin, or any URL understood by uio (gzip, S3, ...). If the
// source cannot be opened the error is logged and the channel closes empty.
func Lines(ctx context.Context, path string) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		f, err := uio.Open(path)
		if err != nil {
			slog.Error("gojob: cannot open source", slog.String("path", path), slog.String("error", err.Error()))
			return
		}
		defer f.Close()
		scanner := bufio.NewScanner(f.(io.Reader))
		for scanner.Scan() {
			select {
			case out <- strings.TrimSpace(scanner.Text()):
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			slog.Error("gojob: error reading source", slog.String("path", path), slog.String("error", err.Error()))
		}
	}()
	return out
}
