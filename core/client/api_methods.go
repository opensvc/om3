package client

import (
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/client/request"
)

// HasRequester returns true if the client has a requester defined.
func (t T) HasRequester() bool {
	return t.requester != nil
}

// GetStream wraps the requester's GetStream method
func (t T) GetStream(req request.T) (chan []byte, error) {
	log.Debug().Msgf("GETSTREAM %s via %s", req, t.requester)
	return t.requester.GetStream(req)
}

// Get wraps the requester's Get method
func (t T) Get(req request.T) ([]byte, error) {
	log.Debug().Msgf("GET %s via %s", req, t.requester)
	return parse(t.requester.Get(req))
}

// Post wraps the requester's Post method
func (t T) Post(req request.T) ([]byte, error) {
	log.Debug().Msgf("POST %s via %s", req, t.requester)
	return parse(t.requester.Post(req))
}

// Put wraps the requester's Put method
func (t T) Put(req request.T) ([]byte, error) {
	log.Debug().Msgf("PUT %s via %s", req, t.requester)
	return parse(t.requester.Put(req))
}

// Delete wraps the requester's Delete method
func (t T) Delete(req request.T) ([]byte, error) {
	log.Debug().Msgf("DELETE %s via %s", req, t.requester)
	return parse(t.requester.Delete(req))
}
