package version

import (
	"fmt"
	"os"
	"time"
)

var (
	Version string = "unknown"
	Commit  string = "unknown"
	Date    string = time.Now().Format("2006-01-02")
)

func GetVersion() string {
	return fmt.Sprintf("Version: %s\nCommit: %s\nDate: %s\n", Version, Commit, Date)
}

func PrintVersion() {
	os.Stderr.WriteString(GetVersion())
	os.Exit(0)
}
