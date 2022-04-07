/*
	Package muxresponse provides Response that implement http.ResponseWriter
*/
package routeresponse

import (
	"bytes"
	"io"
	"net/http"
)

type (
	// Response struct that implement http.ResponseWriter
	Response struct {
		*http.Response

		// writer is holder for raw conn writer
		writer io.Writer
	}

	byteBuffer struct {
		*bytes.Buffer
	}
)

func NewResponse(w io.Writer) *Response {
	return &Response{
		Response: &http.Response{
			StatusCode: 200,
			Header:     make(map[string][]string),
		},
		writer: w,
	}
}

// NewByteResponse() returns *Response where writer and Body is a bytes.Buffer{}
func NewByteResponse() *Response {
	body := byteBuffer{Buffer: &bytes.Buffer{}}
	response := NewResponse(body)
	response.Response.Body = body
	return response
}

// Header function implements http.ResponseWriter Header() for Response
func (resp Response) Header() http.Header {
	return resp.Response.Header
}

// Write function implements Write([]byte) (int, error) for Response
func (resp *Response) Write(b []byte) (int, error) {
	return resp.writer.Write(b)
}

// WriteHeader function implements WriteHeader(int) for Response
func (resp *Response) WriteHeader(statusCode int) {
	resp.StatusCode = statusCode
}

// Close function implements Close() error for byteBuffer
func (b byteBuffer) Close() error {
	b.Reset()
	return nil
}
