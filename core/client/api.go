package client

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
)

type (
	// API abstracts the requester and exposes the agent API methods
	API struct {
		requester Requester `json:"requester"`
	}
)

// HasRequester returns true if the API has a requester defined.
func (t API) HasRequester() bool {
	return t.requester != nil
}

func (t API) String() string {
	b, _ := json.Marshal(t)
	return "API" + string(b)
}

// Get wraps the requester's Get method
func (t API) Get(req Request) ([]byte, error) {
	log.Debug().Msgf("GET %s via %s", req, t.requester)
	return t.requester.Get(req)
}

// GetStream wraps the requester's GetStream method
func (t API) GetStream(req Request) (chan []byte, error) {
	log.Debug().Msgf("GETSTREAM %s via %s", req, t.requester)
	return t.requester.GetStream(req)
}

// Post wraps the requester's Post method
func (t API) Post(req Request) ([]byte, error) {
	log.Debug().Msgf("POST %s via %s", req, t.requester)
	return t.requester.Post(req)
}

// Put wraps the requester's Put method
func (t API) Put(req Request) ([]byte, error) {
	log.Debug().Msgf("PUT %s via %s", req, t.requester)
	return t.requester.Put(req)
}

// Delete wraps the requester's Delete method
func (t API) Delete(req Request) ([]byte, error) {
	log.Debug().Msgf("DELETE %s via %s", req, t.requester)
	return t.requester.Delete(req)
}
