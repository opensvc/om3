package event

import (
	"encoding/json"

	"github.com/pkg/errors"
)

type (
	// Event describes a opensvc daemon event
	Event struct {
		Kind      string           `json:"kind"`
		ID        uint64           `json:"id"`
		Timestamp float64          `json:"ts"`
		Data      *json.RawMessage `json:"data"`
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
