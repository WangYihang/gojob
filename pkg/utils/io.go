package utils

import (
	"bufio"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Head takes a channel and returns a channel with the first n items
func Head[T interface{}](in <-chan T, max int) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		i := 0
		for line := range in {
			if i >= max {
				break
			}
			out <- line
			i++
		}
	}()
	return out
}

// Tail takes a channel and returns a channel with the last n items
func Tail[T interface{}](in <-chan T, max int) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		var lines []T
		for line := range in {
			lines = append(lines, line)
			if len(lines) > max {
				lines = lines[1:]
			}
		}
		for _, line := range lines {
			out <- line
		}
	}()
	return out
}

// Skip takes a channel and returns a channel with the first n items skipped
func Skip[T any](in <-chan T, n int) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for i := 0; i < n; i++ {
			<-in
		}
		for line := range in {
			out <- line
		}
	}()
	return out
}

// Cat takes a file path and returns a channel with the lines of the file
// Spaces are trimmed from the beginning and end of each line
func Cat(filePath string) <-chan string {
	out := make(chan string)

	go func() {
		defer close(out) // Ensure the channel is closed when the goroutine finishes

		// Open the file
		file, err := OpenFile(filePath)
		if err != nil {
			slog.Error("error occurred while opening file", slog.String("path", filePath), slog.String("error", err.Error()))
			return // Close the channel and exit the goroutine
		}
		defer file.Close()

		scanner := bufio.NewScanner(file.(io.Reader)) // Change the type of file to io.Reader
		for scanner.Scan() {
			out <- strings.TrimSpace(scanner.Text()) // Send the line to the channel
		}

		// Check for errors during Scan, excluding EOF
		if err := scanner.Err(); err != nil {
			slog.Error("error occurred while reading file", slog.String("path", filePath), slog.String("error", err.Error()))
		}
	}()

	return out
}

// Count takes a channel and returns the number of items
func Count[T any](in <-chan T) (count int64) {
	for range in {
		count++
	}
	return count
}

type ReadDiscardCloser struct {
	io.Writer
	io.Reader
}

func (wc ReadDiscardCloser) Close() error {
	return nil
}

func OpenFile(path string) (io.WriteCloser, error) {
	switch path {
	case "-":
		return ReadDiscardCloser{Writer: os.Stdout, Reader: os.Stdin}, nil
	case "":
		return ReadDiscardCloser{Writer: io.Discard}, nil
	default:
		// Create folder
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		// Open file
		return os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	}
}
