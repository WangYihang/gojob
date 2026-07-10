package version_test

import (
	"strings"
	"testing"

	"github.com/WangYihang/gojob/pkg/version"
)

func TestGetVersion(t *testing.T) {
	version.Version = "1.2.3"
	version.Commit = "abc123"
	version.Date = "2026-01-01"

	got := version.GetVersion()
	for _, want := range []string{"1.2.3", "abc123", "2026-01-01"} {
		if !strings.Contains(got, want) {
			t.Errorf("GetVersion() = %q, want it to contain %q", got, want)
		}
	}
}
