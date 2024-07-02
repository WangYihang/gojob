package utils

import "net/url"

func ParseProtocol(uri string) (protocol string, err error) {
	parsed, err := url.Parse(uri)
	return parsed.Scheme, err
}
