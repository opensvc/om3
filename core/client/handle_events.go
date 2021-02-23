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
		Full:           true,
	}
}

// Events fetchs an Event stream from the agent api
func (a API) Events(o EventsOptions) (chan event.Event, error) {
	opts := a.NewRequest()
	opts.Method = "events"
	opts.Node = "*"
	resp, err := a.Requester.Get(*opts)
	if err != nil {
		return nil, err
	}
	q := make(chan event.Event, 1000)
	go getMessages(q, resp.Body)
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

func getMessages(q chan event.Event, rc io.ReadCloser) {
	scanner := bufio.NewScanner(rc)
	scanner.Split(splitFunc)
	defer rc.Close()
	defer close(q)
	for {
		scanner.Scan()
		e := &event.Event{}
		b := scanner.Bytes()
		if len(b) == 0 {
			break
		}
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
