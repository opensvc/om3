package client

import (
	"github.com/rs/zerolog/log"
)

// HasRequester returns true if the client has a requester defined.
func (t T) HasRequester() bool {
	return t.requester != nil
}

// Get wraps the requester's Get method
func (t T) Get(req Request) ([]byte, error) {
	log.Debug().Msgf("GET %s via %s", req, t.requester)
	return t.requester.Get(req)
}

// GetStream wraps the requester's GetStream method
func (t T) GetStream(req Request) (chan []byte, error) {
	log.Debug().Msgf("GETSTREAM %s via %s", req, t.requester)
	return t.requester.GetStream(req)
}

// Post wraps the requester's Post method
func (t T) Post(req Request) ([]byte, error) {
	log.Debug().Msgf("POST %s via %s", req, t.requester)
	return t.requester.Post(req)
}

// Put wraps the requester's Put method
func (t T) Put(req Request) ([]byte, error) {
	log.Debug().Msgf("PUT %s via %s", req, t.requester)
	return t.requester.Put(req)
}

// Delete wraps the requester's Delete method
func (t T) Delete(req Request) ([]byte, error) {
	log.Debug().Msgf("DELETE %s via %s", req, t.requester)
	return t.requester.Delete(req)
}
