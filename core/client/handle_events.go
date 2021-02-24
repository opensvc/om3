package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

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
func (a API) EventsRaw(o EventsOptions) (chan interface{}, error) {
	return a.eventsBase(o, true)
}

// Events fetchs an Event stream from the agent api
func (a API) Events(o EventsOptions) (chan interface{}, error) {
	return a.eventsBase(o, false)
}

func (a API) eventsBase(o EventsOptions, raw bool) (chan interface{}, error) {
	req := a.NewRequest()
	req.Action = "events"
	req.Options["selector"] = o.ObjectSelector
	req.Options["namespace"] = o.Namespace
	req.Options["full"] = o.Full
	resp, err := a.Requester.Get(*req)
	if err != nil {
		return nil, err
	}
	q := make(chan interface{}, 1000)
	go getMessages(q, resp.Body, raw)
	return q, nil
}

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

func splitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// That means we've scanned to the end.
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	// Find the location of '\x00'
	if i := bytes.IndexByte(data, '\x00'); i >= 0 {
		// Move I + 1 bit forward from the next start of reading
		return i + 1, dropCR(data[0:i]), nil
	}
	// The reader contents processed here are all read out, but the contents are not empty, so the remaining data needs to be returned.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	// Represents that you can't split up now, and requests more data from Reader
	return 0, nil, nil
}

func getMessages(q chan interface{}, rc io.ReadCloser, raw bool) {
	scanner := bufio.NewScanner(rc)
	min := 1000     // usual event size
	max := 10000000 // max kind=full event size
	scanner.Buffer(make([]byte, min, max), max)
	scanner.Split(splitFunc)
	defer rc.Close()
	defer close(q)
	for {
		scanner.Scan()
		b := scanner.Bytes()
		if len(b) == 0 {
			break
		}
		if raw {
			q <- b
			continue
		}
		e := &event.Event{}
		if err := json.Unmarshal(b, &e); err != nil {
			//fmt.Println("Event stream parse error:", err, string(b))
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if e.Kind == "" {
			fmt.Println("unexpected message:", string(b))
			time.Sleep(100 * time.Millisecond)
			continue
		}
		q <- *e
	}
}
