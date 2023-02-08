package api

import (
	"github.com/opensvc/om3/core/client/request"
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
	req := request.NewFor(t)
	if t.Nodename != "" {
		req.Values.Set("nodename", t.Nodename)
	}
	if t.ClusterId != "" {
		req.Values.Set("cluster_id", t.ClusterId)
	}
	return Route(t.client, *req)
}
