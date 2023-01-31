package api

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/client/request"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/event/sseevent"
)

// GetEvents describes the events request options.
type GetEvents struct {
	client    GetStreamReader
	namespace string
	selector  string
	relatives bool
	Limit     uint64
	Filters   []string
	Duration  time.Duration
}

func (t *GetEvents) SetDuration(duration time.Duration) *GetEvents {
	t.Duration = duration
	return t
}

func (t *GetEvents) SetLimit(limit uint64) *GetEvents {
	t.Limit = limit
	return t
}

func (t *GetEvents) SetFilters(filters []string) *GetEvents {
	t.Filters = filters
	return t
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
func NewGetEvents(t GetStreamReader) *GetEvents {
	options := &GetEvents{
		client:    t,
		namespace: "*",
		selector:  "",
		relatives: true,
	}
	return options
}

// GetRaw fetchs an event json RawMessage stream from the agent api
func (t GetEvents) GetRaw() (chan []byte, error) {
	return t.eventsBase()
}

// Do fetchs an Event stream from the agent api
func (t GetEvents) Do() (chan event.Event, error) {
	// TODO add a stopper to allow GetReader clients to stop fetching event streams retries

	out := make(chan event.Event, 1000)
	errChan := make(chan error)

	go func() {
		defer close(out)
		defer close(errChan)
		hasRunOnce := false
		for {
			q, err := t.eventsBase()
			if err != nil {
				if !hasRunOnce {
					// Notify initial create request failure
					errChan <- err
				}
				return
			}
			if !hasRunOnce {
				hasRunOnce = true
				errChan <- nil
			}
			marshalMessages(q, out)
		}
	}()
	err := <-errChan
	return out, err
}

// GetReader returns event.ReadCloser for GetEventReader
func (t *GetEvents) GetReader() (evReader event.ReadCloser, err error) {
	var r io.ReadCloser
	req := t.newRequest()
	r, err = t.client.GetReader(*req)
	if err != nil {
		return
	}
	evReader = sseevent.NewReadCloser(r)
	return
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
			log.Error().Err(err).Msgf("can't unmarshal '%s' has Event", b)
			continue
		}
		out <- *e
	}
}

func (t GetEvents) eventsBase() (chan []byte, error) {
	req := t.newRequest()
	return t.client.GetStream(*req)
}

func (t GetEvents) newRequest() *request.T {
	req := request.New()
	req.Action = "daemon/events"
	req.Options["selector"] = t.selector
	req.Options["namespace"] = t.namespace
	req.Options["full"] = t.relatives
	if t.Limit > 0 {
		req.Values.Add("limit", fmt.Sprintf("%d", t.Limit))
	}
	for _, filter := range t.Filters {
		req.Values.Add("filter", filter)
	}
	if t.Duration > 0 {
		req.Values.Add("duration", t.Duration.String())
	}
	return req
}
