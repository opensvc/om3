package api

import (
	"opensvc.com/opensvc/core/client/request"
)

type PostRelayMessage struct {
	Base
	ClusterId   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	Msg         string `json:"msg"`
	Nodename    string `json:"nodename"`
}

func NewPostRelayMessage(t Poster) *PostRelayMessage {
	r := &PostRelayMessage{}
	r.SetClient(t)
	r.SetMethod("POST")
	r.SetAction("/relay/message")
	return r
}

func (t PostRelayMessage) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
