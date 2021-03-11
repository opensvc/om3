package client

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
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

func (t API) Get(req Request) ([]byte, error) {
	log.Debugf("GET %s via %s", req, t.requester)
	return t.requester.Get(req)
}

func (t API) GetStream(req Request) (chan []byte, error) {
	log.Debugf("GETSTREAM %s via %s", req, t.requester)
	return t.requester.GetStream(req)
}

func (t API) Post(req Request) ([]byte, error) {
	log.Debugf("POST %s via %s", req, t.requester)
	return t.requester.Post(req)
}

func (t API) Put(req Request) ([]byte, error) {
	log.Debugf("PUT %s via %s", req, t.requester)
	return t.requester.Put(req)
}

func (t API) Delete(req Request) ([]byte, error) {
	log.Debugf("DELETE %s via %s", req, t.requester)
	return t.requester.Delete(req)
}
