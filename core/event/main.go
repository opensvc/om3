package event

import (
	"context"
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
		// cluster dataset by:
		// installing a full dataset with received full dataset
		// or
		// patching a full dataset with received patch events
		Kind string `json:"kind"`

		// ID is a unique event id
		ID uint64 `json:"id"`

		// Time is the time the event was published
		Time time.Time `json:"t"`

		// Data is the free-format dataset of the event
		Data json.RawMessage `json:"data"`
	}

	Reader interface {
		Read() (*Event, error)
	}

	ReadCloser interface {
		Reader
		Close() error
	}

	Kinder interface {
		Kind() string
	}

	Byter interface {
		Bytes() []byte
	}

	Stringer interface {
		String() string
	}

	Timer interface {
		Time() time.Time
	}
)

// ChanFromAny returns event chan from dequeued any chan
func ChanFromAny(ctx context.Context, anyC <-chan any) <-chan *Event {
	eventC := make(chan *Event)
	go func() {
		eventCount := uint64(0)
		for {
			select {
			case <-ctx.Done():
				close(eventC)
				return
			case i := <-anyC:
				switch o := i.(type) {
				case Kinder:
					eventCount++
					ev := &Event{
						Kind: o.Kind(),
						ID:   eventCount,
					}

					if o, ok := i.(Timer); ok {
						ev.Time = o.Time()
					}

					if o, ok := i.(Byter); ok {
						ev.Data = o.Bytes()
					} else {
						ev.Data, _ = json.Marshal(i)
					}
					eventC <- ev
				}
			}
		}
	}()

	return eventC
}
