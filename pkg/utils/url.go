package utils

import "net/url"

func ParseProtocol(uri string) (protocol string, err error) {
	parsed, err := url.Parse(uri)
	if parsed.Scheme == "" {
		return "file", nil
	}
	return parsed.Scheme, err
}
