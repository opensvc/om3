package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"time"
)

// EventsCmdOptions describes the events api handler options.
type EventsCmdOptions struct {
	Namespace      string `json:"namespace"`
	ObjectSelector string `json:"selector"`
	Full           bool   `json:"full"`
}

// NewEventsCmdConfig allocates a EventsCmdConfig struct and sets
// default values to its keys.
func NewEventsCmdConfig() *EventsCmdOptions {
	return &EventsCmdOptions{
		Namespace:      "*",
		ObjectSelector: "**",
		Full:           true,
	}
}

// Event describes a opensvc daemon event
type Event struct {
	Kind      string      `json:"kind"`
	ID        uint64      `json:"id"`
	Timestamp float64     `json:"ts"`
	Data      interface{} `json:"data"`
}

// Events fetchs an Event stream from the agent api
func (a API) Events(o EventsCmdOptions) (chan Event, error) {
	opts := a.NewRequestOptions()
	opts.Node = "*"
	resp, err := a.Requester.Get("events", *opts)
	if err != nil {
		return nil, err
	}
	q := make(chan Event, 1000)
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

func getMessages(q chan Event, rc io.ReadCloser) {
	scanner := bufio.NewScanner(rc)
	scanner.Split(splitFunc)
	defer rc.Close()
	for {
		scanner.Scan()
		e := &Event{}
		b := scanner.Bytes()
		if err := json.Unmarshal(b, &e); err != nil {
			//fmt.Printf("Event stream parse error: %s", err)
			time.Sleep(100 * time.Millisecond)
		}
		q <- *e
	}
}
