package getevent

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/event"
)

// GetEvents describes the events request options.
type getEvent struct {
	cli       client.GetStreamer `json:"-"`
	namespace string             `json:"namespace"`
	selector  string             `json:"selector"`
	full      bool               `json:"full"`
}

// NewGetEvents allocates a EventsCmdConfig struct and sets
// default values to its keys.
func New(cli client.GetStreamer, selector string, full bool) *getEvent {
	return &getEvent{
		cli:       cli,
		namespace: "*",
		selector:  selector,
		full:      full,
	}
}

// DoRaw fetchs an event json RawMessage stream from the agent api
func (o getEvent) GetRaw() (chan []byte, error) {
	return o.eventsBase()
}

// Do fetchs an Event stream from the agent api
func (o getEvent) Do() (chan event.Event, error) {
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

func (o getEvent) eventsBase() (chan []byte, error) {
	req := o.newRequest()
	return o.cli.GetStream(*req)
}

func (o getEvent) newRequest() *client.Request {
	request := client.NewRequest()
	request.Action = "events"
	request.Options["selector"] = o.selector
	request.Options["namespace"] = o.namespace
	request.Options["full"] = o.full
	return request
}
