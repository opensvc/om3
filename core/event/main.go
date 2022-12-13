package event

import (
	"encoding/json"
	"time"
)

type (
	// Event describes a opensvc daemon event
	Event struct {
		// Kind can be either "patch" or "event".
		// A patch is a change to the cluster dataset.
		//
		// Event subscribers can maintain a clone of the
		// cluster dataset by patching a full with received
		// patch events.
		Kind string `json:"kind"`

		// ID is a unique event id
		ID uint64 `json:"id"`

		// Time is the time the event was published
		Time time.Time `json:"t"`

		// Data is the free-format dataset of the event
		Data json.RawMessage `json:"data"`
	}

	ReadCloser interface {
		Read() (*Event, error)
		Close() error
	}
)
