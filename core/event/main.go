package event

import (
	"encoding/json"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/timestamp"
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

		// Timestamp is the time the event was published
		Timestamp timestamp.T `json:"ts"`

		// Data is the free-format dataset of the event
		Data *json.RawMessage `json:"data"`
	}
)

var (
	// ErrInvalidKind signals the event message as the "kind" key set
	// to an invalid value (not event nor patch)
	ErrInvalidKind = errors.New("unexpected event kind")
)

// DecodeFromJSON parses a json message and returns a configured Event
func DecodeFromJSON(b json.RawMessage) (Event, error) {
	e := Event{}
	if err := json.Unmarshal(b, &e); err != nil {
		return e, err
	}
	if e.Kind == "" {
		return e, errors.Wrapf(ErrInvalidKind, "%s", string(b))
	}
	return e, nil
}
