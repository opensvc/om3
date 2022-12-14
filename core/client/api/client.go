package api

import (
	"fmt"
	"io"

	"opensvc.com/opensvc/core/client/request"
)

type (
	GetReader interface {
		GetReader(r request.T) (reader io.ReadCloser, err error)
	}

	GetStreamer interface {
		GetStream(r request.T) (chan []byte, error)
	}

	GetStreamReader interface {
		GetReader
		GetStreamer
	}

	Getter interface {
		Get(r request.T) ([]byte, error)
	}
	Poster interface {
		Post(r request.T) ([]byte, error)
	}
	Putter interface {
		Put(r request.T) ([]byte, error)
	}
	Deleter interface {
		Delete(r request.T) ([]byte, error)
	}

	// Requester abstracts the requesting details of supported protocols
	Requester interface {
		Getter
		Poster
		Putter
		Deleter
		GetStreamer
		GetReader
	}
)

// Route submits the request via a requester
func Route(requester Requester, req request.T) ([]byte, error) {
	switch req.Method {
	case "GET":
		return requester.Get(req)
	case "POST":
		return requester.Post(req)
	case "PUT":
		return requester.Put(req)
	case "DELETE":
		return requester.Delete(req)
	default:
		return nil, fmt.Errorf("unsupported method: %s", req.Method)
	}
}
