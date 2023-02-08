package api

import (
	"github.com/opensvc/om3/core/client/request"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/slog"
)

// GetObjectsLog describes the node log request options.
type GetObjectsLog struct {
	client  GetStreamer
	Filters map[string]interface{} `json:"filters"`
	Paths   path.L                 `json:"paths"`
}

// NewGetObjectsLog allocates a GetObjectsLog struct and sets
// default values to its keys.
func NewGetObjectsLog(t GetStreamer) *GetObjectsLog {
	options := &GetObjectsLog{
		client:  t,
		Filters: make(map[string]interface{}),
		Paths:   make(path.L, 0),
	}
	return options
}

// Do fetchs an Event stream from the agent api
func (t GetObjectsLog) Do() (chan slog.Event, error) {
	out := make(chan slog.Event, 1000)
	q, err := t.stream()
	if err != nil {
		return out, err
	}
	go func() {
		defer close(out)
		for m := range q {
			marshalMessage(m, out)
		}
	}()
	return out, err
}

func (t GetObjectsLog) stream() (chan []byte, error) {
	req := t.newRequest()
	return t.client.GetStream(*req)
}

func (t GetObjectsLog) newRequest() *request.T {
	req := request.New()
	req.Action = "objects_log"
	req.Options["filters"] = t.Filters
	req.Options["paths"] = t.Paths
	return req
}
