package main

import (
	"log/slog"

	"github.com/WangYihang/gojob/pkg/utils"
)

func main() {
	for line := range utils.Cat(
		"s3://uio/example.txt" +
			"?endpoint=127.0.0.1:9000" +
			"&access_key=********************" +
			"&secret_key=********************************************" +
			"&insecure=true" +
			"&download=true",
	) {
		slog.Info("s3", slog.String("line", line))
	}
}
