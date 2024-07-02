package main

import (
	"log/slog"

	"github.com/WangYihang/gojob/pkg/utils"
)

func main() {
	for line := range utils.Cat(
		"s3://example/shakespeare.txt" +
			"?region=us-west-1" +
			"&bucket=default" +
			"&access_key=********************" +
			"&secret_key=********************************************",
	) {
		slog.Info("s3", slog.String("line", line))
	}
}
