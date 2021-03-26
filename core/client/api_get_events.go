package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/event"
)

// GetEvents describes the events request options.
type getEvent struct {
	cli GetStreamer `json:"-"`
	*Namespace
	*Selector
	*Relatives
}

// NewGetEvents allocates a EventsCmdConfig struct and sets
// default values to its keys.
func NewGetEvents(cli GetStreamer, opts ...OptionExtra) (*getEvent, error) {
	options := getEvent{
		cli,
		&Namespace{"*"},
		&Selector{""},
		&Relatives{true},
	}
	for _, o := range opts {
		switch t := o.(type) {
		case SelectorType:
			_ = t.apply(options.Selector)
		case NamespaceType:
			_ = t.apply(options.Namespace)
		case RelativesType:
			_ = t.apply(options.Relatives)
		default:
			message := fmt.Sprintf("non allowed option type %T", t)
			return nil, errors.New(message)
		}
	}
	return &options, nil
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

func (o getEvent) newRequest() *Request {
	request := NewRequest()
	request.Action = "events"
	request.Options["selector"] = o.SelectorValue()
	request.Options["namespace"] = o.NamespaceValue()
	request.Options["full"] = o.RelativesValue()
	return request
}
