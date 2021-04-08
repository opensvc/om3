package client

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/util/funcopt"
)

// GetEvents describes the events request options.
type getEvents struct {
	cli       GetStreamer `json:"-"`
	namespace string
	selector  string
	relatives bool
}

func (t *getEvents) SetNamespace(s string) {
	t.namespace = s
}

func (t *getEvents) SetSelector(s string) {
	t.selector = s
}

func (t *getEvents) SetRelatives(s bool) {
	t.relatives = s
}

// NewGetEvents allocates a EventsCmdConfig struct and sets
// default values to its keys.
func NewGetEvents(cli GetStreamer, opts ...funcopt.O) (*getEvents, error) {
	options := &getEvents{
		cli:       cli,
		namespace: "*",
		selector:  "",
		relatives: true,
	}
	funcopt.Apply(options, opts...)
	return options, nil
}

// DoRaw fetchs an event json RawMessage stream from the agent api
func (o getEvents) GetRaw() (chan []byte, error) {
	return o.eventsBase()
}

// Do fetchs an Event stream from the agent api
func (o getEvents) Do() (chan event.Event, error) {
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

func (o getEvents) eventsBase() (chan []byte, error) {
	req := o.newRequest()
	return o.cli.GetStream(*req)
}

func (o getEvents) newRequest() *Request {
	request := NewRequest()
	request.Action = "events"
	request.Options["selector"] = o.selector
	request.Options["namespace"] = o.namespace
	request.Options["full"] = o.relatives
	return request
}
