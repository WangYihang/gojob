package model

import (
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
)

type HTTP struct {
	Request  *HTTPRequest  `json:"request"`
	Response *HTTPResponse `json:"response"`
}

type HTTPRequest struct {
	Method string `json:"method"`
	URL    string `json:"url"`
	Host   string `json:"host"`

	RemoteAddr string `json:"remote_addr"`
	RequestURI string `json:"request_uri"`

	Proto      string `json:"proto"`
	ProtoMajor int    `json:"proto_major"`
	ProtoMinor int    `json:"proto_minor"`

	Header http.Header `json:"header"`

	ContentLength    int64    `json:"content_length"`
	TransferEncoding []string `json:"transfer_encoding"`
	Close            bool     `json:"close"`

	Form          url.Values      `json:"form"`
	PostForm      url.Values      `json:"post_form"`
	MultipartForm *multipart.Form `json:"multipart_form"`

	Trailer http.Header `json:"trailer"`
}

func NewHTTPRequest(req *http.Request) (*HTTPRequest, error) {
	httpRequest := &HTTPRequest{
		Method:           req.Method,
		URL:              req.URL.String(),
		Host:             req.Host,
		RemoteAddr:       req.RemoteAddr,
		RequestURI:       req.RequestURI,
		Proto:            req.Proto,
		ProtoMajor:       req.ProtoMajor,
		ProtoMinor:       req.ProtoMinor,
		Header:           req.Header,
		ContentLength:    req.ContentLength,
		TransferEncoding: req.TransferEncoding,
		Close:            req.Close,
		Form:             req.Form,
		PostForm:         req.PostForm,
		MultipartForm:    req.MultipartForm,
		Trailer:          req.Trailer,
	}
	return httpRequest, nil
}

type HTTPResponse struct {
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`

	Proto      string `json:"proto"`
	ProtoMajor int    `json:"proto_major"`
	ProtoMinor int    `json:"proto_minor"`

	Header http.Header `json:"header"`

	Body []byte `json:"body"`

	ContentLength    int64       `json:"content_length"`
	TransferEncoding []string    `json:"transfer_encoding"`
	Close            bool        `json:"close"`
	Uncompressed     bool        `json:"uncompressed"`
	Trailer          http.Header `json:"trailer"`
}

func NewHTTPResponse(resp *http.Response) (*HTTPResponse, error) {
	httpResponse := &HTTPResponse{
		Status:           resp.Status,
		StatusCode:       resp.StatusCode,
		Proto:            resp.Proto,
		ProtoMajor:       resp.ProtoMajor,
		ProtoMinor:       resp.ProtoMinor,
		Header:           resp.Header,
		Body:             []byte{},
		ContentLength:    resp.ContentLength,
		TransferEncoding: resp.TransferEncoding,
		Close:            resp.Close,
		Uncompressed:     resp.Uncompressed,
		Trailer:          resp.Trailer,
	}
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("error occured while reading response body", slog.String("error", err.Error()))
		return httpResponse, nil
	}
	httpResponse.Body = body
	return httpResponse, nil
}
