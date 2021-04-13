package api

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/client/request"
	"opensvc.com/opensvc/core/event"
)

// GetEvents describes the events request options.
type GetEvents struct {
	client    GetStreamer
	namespace string
	selector  string
	relatives bool
}

func (t *GetEvents) SetNamespace(s string) *GetEvents {
	t.namespace = s
	return t
}

func (t *GetEvents) SetSelector(s string) *GetEvents {
	t.selector = s
	return t
}

func (t *GetEvents) SetRelatives(s bool) *GetEvents {
	t.relatives = s
	return t
}

func (t GetEvents) Namespace() string {
	return t.namespace
}

func (t GetEvents) Selector() string {
	return t.selector
}

func (t GetEvents) Relatives() bool {
	return t.relatives
}

// NewGetEvents allocates a EventsCmdConfig struct and sets
// default values to its keys.
func NewGetEvents(t GetStreamer) *GetEvents {
	options := &GetEvents{
		client:    t,
		namespace: "*",
		selector:  "",
		relatives: true,
	}
	return options
}

// GetRaw fetchs an event json RawMessage stream from the agent api
func (o GetEvents) GetRaw() (chan []byte, error) {
	return o.eventsBase()
}

// Do fetchs an Event stream from the agent api
func (o GetEvents) Do() (chan event.Event, error) {
	q, err := o.eventsBase()
	if err != nil {
		return nil, err
	}
	out := make(chan event.Event, 1000)
	go marshalMessages(q, out)
	return out, nil
}

func marshalMessages(q chan []byte, out chan event.Event) {
	var (
		b  []byte
		ok bool
	)
	for {
		b, ok = <-q
		if !ok {
			break // channel closed
		}
		e := &event.Event{}
		if err := json.Unmarshal(b, e); err != nil {
			log.Error().Err(err).Msg("")
			continue
		}
		out <- *e
	}
}

func (o GetEvents) eventsBase() (chan []byte, error) {
	req := o.newRequest()
	return o.client.GetStream(*req)
}

func (o GetEvents) newRequest() *request.T {
	req := request.New()
	req.Action = "events"
	req.Options["selector"] = o.selector
	req.Options["namespace"] = o.namespace
	req.Options["full"] = o.relatives
	return req
}
