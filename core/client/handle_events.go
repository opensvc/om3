package client

import (
	"encoding/json"
	"fmt"
	"os"

	"opensvc.com/opensvc/core/event"
)

// EventsOptions describes the events api handler options.
type EventsOptions struct {
	Namespace      string `json:"namespace"`
	ObjectSelector string `json:"selector"`
	Full           bool   `json:"full"`
}

// NewEventsOptions allocates a EventsCmdConfig struct and sets
// default values to its keys.
func NewEventsOptions() *EventsOptions {
	return &EventsOptions{
		Namespace:      "*",
		ObjectSelector: "**",
		Full:           false,
	}
}

// EventsRaw fetchs an event json RawMessage stream from the agent api
func (a API) EventsRaw(o EventsOptions) (chan []byte, error) {
	return a.eventsBase(o)
}

// Events fetchs an Event stream from the agent api
func (a API) Events(o EventsOptions) (chan event.Event, error) {
	q, err := a.eventsBase(o)
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

func (a API) eventsBase(o EventsOptions) (chan []byte, error) {
	req := a.NewRequest()
	req.Action = "events"
	req.Options["selector"] = o.ObjectSelector
	req.Options["namespace"] = o.Namespace
	req.Options["full"] = o.Full
	return a.Requester.GetStream(*req)
}
