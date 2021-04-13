package api

import "opensvc.com/opensvc/core/client/request"

type (
	Getter interface {
		Get(r request.T) ([]byte, error)
	}

	GetStreamer interface {
		GetStream(r request.T) (chan []byte, error)
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
	}
)
