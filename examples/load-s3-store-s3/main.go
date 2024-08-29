package main

import (
	"log/slog"

	"github.com/WangYihang/gojob/pkg/utils"
	"github.com/WangYihang/uio"
)

func main() {
	resultFd, err := uio.Open("s3://uio/output.txt" +
		"?endpoint=127.0.0.1:9000" +
		"&access_key=minioadmin" +
		"&secret_key=minioadmin" +
		"&insecure=true" +
		"&mode=write",
	)
	if err != nil {
		slog.Error("failed to open result file", slog.String("error", err.Error()))
		return
	}
	defer resultFd.Close()
	for line := range utils.Cat(
		"s3://uio/input.txt" +
			"?endpoint=127.0.0.1:9000" +
			"&access_key=minioadmin" +
			"&secret_key=minioadmin" +
			"&insecure=true" +
			"&mode=read",
	) {
		slog.Info("s3", slog.String("line", line))
		resultFd.Write([]byte(line + "\n"))
	}
}
