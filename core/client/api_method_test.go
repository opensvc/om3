package client

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/core/client/request"
)

type (
	mockRequest struct {
		result []byte
		err    error
	}

	mockReadCloser struct {
		io.Reader
	}
)

func (m mockRequest) Get(request.T) ([]byte, error) {
	return m.doRequest()
}

func (m mockRequest) Post(request.T) ([]byte, error) {
	return m.doRequest()
}

func (m mockRequest) Put(request.T) ([]byte, error) {
	return m.doRequest()
}

func (m mockRequest) Delete(request.T) ([]byte, error) {
	return m.doRequest()
}

func (_ mockRequest) GetStream(request.T) (chan []byte, error) {
	return nil, nil
}

func (m mockRequest) doRequest() ([]byte, error) {
	return m.result, m.err
}

func (m mockRequest) GetReader(r request.T) (reader io.ReadCloser, err error) {
	reader = mockReadCloser{Reader: bytes.NewReader(m.result)}
	return
}

func (rc mockReadCloser) Close() error {
	return nil
}

func TestApiMethods(t *testing.T) {
	t.Run("Ensure parsable result is returned when status is 0 like", func(t *testing.T) {
		c := &T{}
		cases := []struct {
			Name   string
			Method func() func(req request.T) ([]byte, error)
		}{
			{"Get", func() func(req request.T) ([]byte, error) { return c.Get }},
			{"Post", func() func(req request.T) ([]byte, error) { return c.Post }},
			{"Put", func() func(req request.T) ([]byte, error) { return c.Put }},
			{"Delete", func() func(req request.T) ([]byte, error) { return c.Delete }},
		}
		for _, tc := range cases {
			t.Run("method "+tc.Name, func(t *testing.T) {
				subCases := []struct {
					Name  string
					Value string
				}{
					{"0", "0"},
					{"0.0", "0.0"},
					{"0.000", "0.000"},
					{"string 0", "\"0\""},
				}
				for _, tsc := range subCases {
					t.Run("with status "+tsc.Name, func(t *testing.T) {
						result := "{\"status\": " + tsc.Value + ", \"data\": {\"Count\":3,\"Name\":\"foo\"}}"
						c.requester = mockRequest{
							result: []byte(result),
							err:    nil,
						}
						b, err := tc.Method()(request.T{})
						assert.Equal(t, nil, err)
						assert.NotNil(t, b)
						assert.Equal(t, "{\"Count\":3,\"Name\":\"foo\"}", string(b))
					})
				}
			})
		}
	})
}
