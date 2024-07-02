package utils_test

import (
	"testing"

	"github.com/WangYihang/gojob/pkg/utils"
)

func TestParseProtocol(t *testing.T) {
	testcases := []struct {
		uri      string
		expected string
	}{
		{
			uri:      "http://example.com",
			expected: "http",
		},
		{
			uri:      "https://example.com",
			expected: "https",
		},
		{
			uri:      "ftp://example.com",
			expected: "ftp",
		},
		{
			uri:      "file://example.com",
			expected: "file",
		},
		{
			uri:      "s3://bucket/shakespeare.txt",
			expected: "s3",
		},
		{
			uri:      "/etc/passwd",
			expected: "file",
		},
	}
	for _, tc := range testcases {
		protocol, err := utils.ParseProtocol(tc.uri)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if protocol != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, protocol)
		}
	}
}
