package utils

import (
	"bufio"
	"log/slog"
	"os"
	"strings"
)

// Head takes a channel and returns a channel with the first n items
func Head[T interface{}](in chan T, max int) chan T {
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
func Tail[T interface{}](in chan T, max int) chan T {
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

// Cat takes a file path and returns a channel with the lines of the file
// Spaces are trimmed from the beginning and end of each line
func Cat(filePath string) <-chan string {
	out := make(chan string)

	go func() {
		defer close(out) // Ensure the channel is closed when the goroutine finishes

		// Open the file
		file, err := os.Open(filePath)
		if err != nil {
			slog.Error("error occured while opening file", slog.String("path", filePath), slog.String("error", err.Error()))
			return // Close the channel and exit the goroutine
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			out <- strings.TrimSpace(scanner.Text()) // Send the line to the channel
		}

		// Check for errors during Scan, excluding EOF
		if err := scanner.Err(); err != nil {
			slog.Error("error occured while reading file", slog.String("path", filePath), slog.String("error", err.Error()))
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
