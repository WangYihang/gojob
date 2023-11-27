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
	Method string `json:"Method"`
	URL    string `json:"URL"`
	Host   string `json:"Host"`

	RemoteAddr string `json:"RemoteAddr"`
	RequestURI string `json:"RequestURI"`

	Proto      string `json:"Proto"`
	ProtoMajor int    `json:"ProtoMajor"`
	ProtoMinor int    `json:"ProtoMinor"`

	Header http.Header `json:"Header"`

	ContentLength    int64    `json:"ContentLength"`
	TransferEncoding []string `json:"TransferEncoding"`
	Close            bool     `json:"Close"`

	Form          url.Values      `json:"Form"`
	PostForm      url.Values      `json:"PostForm"`
	MultipartForm *multipart.Form `json:"MultipartForm"`

	Trailer http.Header `json:"Trailer"`
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
	Status     string `json:"Status"`
	StatusCode int    `json:"StatusCode"`

	Proto      string `json:"Proto"`
	ProtoMajor int    `json:"ProtoMajor"`
	ProtoMinor int    `json:"ProtoMinor"`

	Header http.Header `json:"Header"`

	Body []byte `json:"Body"`

	ContentLength    int64       `json:"ContentLength"`
	TransferEncoding []string    `json:"TransferEncoding"`
	Close            bool        `json:"Close"`
	Uncompressed     bool        `json:"Uncompressed"`
	Trailer          http.Header `json:"Trailer"`
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
