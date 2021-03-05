package client

import (
	"encoding/json"
	"fmt"
	"os"

	"opensvc.com/opensvc/core/event"
)

// GetEvents describes the events request options.
type GetEvents struct {
	API            API    `json:"-"`
	Namespace      string `json:"namespace"`
	ObjectSelector string `json:"selector"`
	Full           bool   `json:"full"`
}

// NewGetEvents allocates a EventsCmdConfig struct and sets
// default values to its keys.
func (a API) NewGetEvents() *GetEvents {
	return &GetEvents{
		API:            a,
		Namespace:      "*",
		ObjectSelector: "**",
		Full:           false,
	}
}

// DoRaw fetchs an event json RawMessage stream from the agent api
func (o GetEvents) DoRaw() (chan []byte, error) {
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
		b   []byte
		e   event.Event
		err error
		ok  bool
	)
	for {
		b, ok = <-q
		if !ok {
			break // channel closed
		}
		err = json.Unmarshal(b, &e)
		if err != nil {
			fmt.Fprintln(os.Stderr, "marshal raw event messages:", err)
			continue
		}
		out <- e
	}
}

func (o GetEvents) eventsBase() (chan []byte, error) {
	req := NewRequest()
	req.Action = "events"
	req.Options["selector"] = o.ObjectSelector
	req.Options["namespace"] = o.Namespace
	req.Options["full"] = o.Full
	return o.API.Requester.GetStream(*req)
}
