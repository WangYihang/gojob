package utils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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
	protocol, err := ParseProtocol(path)
	if err != nil {
		return nil, err
	}
	switch protocol {
	case "s3":
		slog.Info("opening s3 file", slog.String("path", path))
		return OpenS3File(path)
	case "file":
		slog.Info("opening local file", slog.String("path", path))
		return OpenLocalFile(path)
	default:
		slog.Warn("unsupported protocol", slog.String("protocol", protocol))
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// OpenS3File opens a file from S3
// e.g. s3://default/data.json?region=us-west-1&endpoint=s3.amazonaws.com&access_key=********************&secret_key=****************************************
func OpenS3File(path string) (io.WriteCloser, error) {
	// Parse the path
	parsed, _ := url.Parse(path)
	bucketName := parsed.Host
	objectKey := strings.TrimLeft(parsed.Path, "/")
	query := parsed.Query()
	endpoint := query.Get("endpoint")
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}
	accessKey := query.Get("access_key")
	secretKey := query.Get("secret_key")
	region := query.Get("region")
	slog.Info(
		"parsed s3 path",
		slog.String("access_key", accessKey),
		slog.String("secret_key", secretKey),
		slog.String("bucket", bucketName),
		slog.String("object", objectKey),
		slog.String("endpoint", endpoint),
		slog.String("region", region),
	)

	// Download file from S3 into a temporary file
	slog.Info("downloading file from s3", slog.String("bucket", bucketName), slog.String("object", objectKey))
	s3Client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
		Region: region,
	})
	if err != nil {
		return nil, err
	}
	reader, err := s3Client.GetObject(context.Background(), bucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	fd, err := os.CreateTemp("", "gojob-*")
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	_, err = io.Copy(fd, reader)
	if err != nil {
		return nil, err
	}

	// Open the temporary file
	slog.Info("opening local file", slog.String("path", fd.Name()))
	return OpenLocalFile(fd.Name())
}

func OpenLocalFile(path string) (io.WriteCloser, error) {
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
