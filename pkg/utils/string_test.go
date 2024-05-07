package utils_test

import (
	"fmt"
	"testing"

	"github.com/WangYihang/gojob/pkg/utils"
)

func ExampleSanitize() {
	sanitized := utils.Sanitize("New York")
	fmt.Println(sanitized)
	// Output: new-york
}

func TestSanitize(t *testing.T) {
	testcases := []struct {
		s    string
		want string
	}{
		{s: "New York", want: "new-york"},
		{s: "New-York", want: "new-york"},
		{s: "NewYork", want: "newyork"},
	}
	for _, testcase := range testcases {
		got := utils.Sanitize(testcase.s)
		if got != testcase.want {
			t.Errorf("Sanitize(%#v), want: %#v, expected: %#v", testcase.s, testcase.want, got)
		}
	}
}
