// Command stdio is the smallest possible gojob pipeline: read lines from stdin,
// upper-case each concurrently, and write one JSON result per line to stdout.
package main

import (
	"context"
	"os"
	"strings"

	"github.com/WangYihang/gojob"
)

func main() {
	ctx := context.Background()
	lines := gojob.Lines(ctx, "-")
	results := gojob.Process(ctx, lines, func(ctx context.Context, line string) (string, error) {
		return strings.ToUpper(line), nil
	}, gojob.WithWorkers(4))
	_ = gojob.WriteJSONL(ctx, os.Stdout, results)
}
