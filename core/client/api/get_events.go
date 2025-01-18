package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/event/sseevent"
	"github.com/opensvc/om3/daemon/api"
)

// GetEvents describes the events request options.
type GetEvents struct {
	client    api.ClientInterface
	nodename  string
	namespace *string
	selector  *string
	relatives *bool
	Limit     *uint64
	Filters   []string
	Wait      bool
	Duration  *time.Duration
}

func (t *GetEvents) SetDuration(duration time.Duration) *GetEvents {
	t.Duration = &duration
	return t
}

func (t *GetEvents) SetLimit(limit uint64) *GetEvents {
	t.Limit = &limit
	return t
}

func (t *GetEvents) SetFilters(filters []string) *GetEvents {
	t.Filters = filters
	return t
}

func (t *GetEvents) SetWait(wait bool) *GetEvents {
	t.Wait = wait
	return t
}

func (t *GetEvents) SetNamespace(s string) *GetEvents {
	t.namespace = &s
	return t
}

func (t *GetEvents) SetNodename(s string) *GetEvents {
	t.nodename = s
	return t
}

func (t *GetEvents) SetSelector(s string) *GetEvents {
	t.selector = &s
	return t
}

func (t *GetEvents) SetRelatives(s bool) *GetEvents {
	t.relatives = &s
	return t
}

// NewGetEvents allocates a EventsCmdConfig struct and sets
// default values to its keys.
func NewGetEvents(t api.ClientInterface) *GetEvents {
	options := &GetEvents{
		client:   t,
		nodename: "localhost",
	}
	return options
}

func getServerSideEvents(q chan<- []byte, resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("<nil> event")
	}
	br := bufio.NewReader(resp.Body)
	delim := []byte{':', ' '}
	defer func() {
		_ = resp.Body.Close()
	}()
	for {
		bs, err := br.ReadBytes('\n')

		if err != nil {
			return err
		}

		if len(bs) < 2 {
			continue
		}

		spl := bytes.Split(bs, delim)

		if len(spl) < 2 {
			continue
		}

		switch string(spl[0]) {
		case "data":
			b := bytes.TrimLeft(bs, "data: ")
			q <- b
		}
		if err == io.EOF {
			break
		}
	}
	return nil
}

// GetRaw fetchs an event json RawMessage stream from the agent api
func (t GetEvents) GetRaw() (chan []byte, error) {
	resp, err := t.eventsBase()
	if err != nil {
		return nil, err
	}

	// TODO add a stopper to allow GetStream clients to stop sse retries
	q := make(chan []byte, 1000)
	delayRestart := 500 * time.Millisecond
	go func() {
		defer close(q)
		_ = getServerSideEvents(q, resp)
		time.Sleep(delayRestart)
	}()
	return q, nil

}

// Do fetchs an Event stream from the agent api
func (t GetEvents) Do() (chan event.Event, error) {
	q, err := t.GetRaw()
	if err != nil {
		return nil, err
	}

	// TODO add a stopper to allow GetReader clients to stop fetching event streams retries
	out := make(chan event.Event, 1000)

	go func() {
		defer close(out)
		for {
			marshalMessages(q, out)
		}
	}()
	return out, nil
}

// GetReader returns event.ReadCloser for GetEventReader
func (t *GetEvents) GetReader() (event.ReadCloser, error) {
	resp, err := t.eventsBase()
	if err != nil {
		return nil, err
	}
	return sseevent.NewReadCloser(resp.Body), nil
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
			continue
		}
		out <- *e
	}
}

func (t GetEvents) eventsBase() (*http.Response, error) {
	params := api.GetDaemonEventsParams{
		Filter:   &t.Filters,
		Selector: t.selector,
		Cache:    &t.Wait,
	}
	if t.Limit != nil {
		i := int64(*t.Limit)
		params.Limit = &i
	}
	if t.Duration != nil {
		s := t.Duration.String()
		params.Duration = &s
	}
	resp, err := t.client.GetDaemonEvents(context.Background(), t.nodename, &params)
	if err != nil {
		return resp, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp, nil
	case http.StatusBadRequest:
	case http.StatusUnauthorized:
	case http.StatusForbidden:
	case http.StatusInternalServerError:
	default:
		return nil, fmt.Errorf("unexpected get events status code %s", resp.Status)
	}
	if b, err := io.ReadAll(resp.Body); err != nil {
		return nil, fmt.Errorf("unexpected get events status code %s", resp.Status)
	} else {
		return nil, fmt.Errorf("unexpected get events status code %s: %s", resp.Status, b)
	}
}
