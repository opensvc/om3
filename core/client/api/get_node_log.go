package api

import (
	"encoding/json"

	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/client/request"
	"github.com/opensvc/om3/core/slog"
)

// GetNodeLog describes the node log request options.
type GetNodeLog struct {
	client  GetStreamer
	Filters map[string]interface{}
}

// NewGetNodeLog allocates a GetNodeLog struct and sets
// default values to its keys.
func NewGetNodeLog(t GetStreamer) *GetNodeLog {
	options := &GetNodeLog{
		client:  t,
		Filters: make(map[string]interface{}),
	}
	return options
}

// Do fetchs an Event stream from the agent api
func (t GetNodeLog) Do() (chan slog.Event, error) {
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

func marshalMessage(m []byte, out chan slog.Event) {
	e := &slog.Event{}
	if err := json.Unmarshal(m, e); err != nil {
		log.Error().Err(err).Str("json", string(m)).Msg("unmarshal log event")
		return
	}
	out <- *e
}

func (t GetNodeLog) stream() (chan []byte, error) {
	req := t.newRequest()
	return t.client.GetStream(*req)
}

func (t GetNodeLog) newRequest() *request.T {
	req := request.New()
	req.Action = "node_log"
	req.Options["filters"] = t.Filters
	return req
}
