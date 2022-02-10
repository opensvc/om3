package api

import (
	"time"

	"opensvc.com/opensvc/core/client/request"
	"opensvc.com/opensvc/core/event"
)

// GetEventsDemo describes the events request options.
type GetEventsDemo struct {
	GetEvents
}

// NewGetEventsDemo allocates a DemoEventsCmdConfig struct and sets
// default values to its keys.
func NewGetEventsDemo(t GetStreamer) *GetEventsDemo {
	options := &GetEventsDemo{
		GetEvents: *NewGetEvents(t),
	}
	return options
}

// Do fetchs an Event stream from the agent api
func (t GetEventsDemo) Do() (chan event.Event, error) {
	// TODO add a stopper to allow Do clients to stop fetching event streams retries

	out := make(chan event.Event, 1000)
	errChan := make(chan error)
	delayRestart := 500 * time.Millisecond

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
			time.Sleep(delayRestart)
		}
	}()
	err := <-errChan
	return out, err
}

func (t GetEventsDemo) eventsBase() (chan []byte, error) {
	req := t.newRequest()
	return t.client.GetStream(*req)
}

func (t GetEventsDemo) newRequest() *request.T {
	req := request.New()
	req.Action = "daemon/eventsdemo"
	req.Options["selector"] = t.selector
	req.Options["namespace"] = t.namespace
	req.Options["full"] = t.relatives
	return req
}
