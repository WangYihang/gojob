package utils

import "strings"

func Sanitize(s string) string {
	builder := strings.Builder{}
	for _, c := range s {
		if 'a' <= c && c <= 'z' {
			builder.WriteRune(c)
			continue
		}
		if 'A' <= c && c <= 'Z' {
			builder.WriteRune(c - 'A' + 'a')
			continue
		}
		if '0' <= c && c <= '9' {
			builder.WriteRune(c)
			continue
		}
		if c == ' ' || c == '-' {
			builder.WriteString("-")
		}
	}
	return builder.String()
}
