package client

import "encoding/json"

type (
	// API abstracts the requester and exposes the agent API methods
	API struct {
		Requester Requester `json:"requester"`
	}
)

func (t API) String() string {
	b, _ := json.Marshal(t)
	return string(b)
}
