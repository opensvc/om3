package api

import (
	"opensvc.com/opensvc/core/client/request"
)

type GetRelayMessage struct {
	Base
	ClusterId string `json:"-"`
	Nodename  string `json:"-"`
}

func NewGetRelayMessage(t Getter) *GetRelayMessage {
	r := &GetRelayMessage{}
	r.SetClient(t)
	r.SetAction("/relay/message")
	r.SetMethod("GET")
	return r
}

func (t GetRelayMessage) Do() ([]byte, error) {
	t.SetQueryArgs(map[string]string{
		"nodename":   t.Nodename,
		"cluster_id": t.ClusterId,
	})
	req := request.NewFor(t)
	return Route(t.client, *req)
}
