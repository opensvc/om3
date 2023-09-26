package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/event/sseevent"
	"github.com/opensvc/om3/daemon/api"
)

// GetLogs describes the events request options.
type GetLogs struct {
	client  api.ClientInterface
	Filters *[]string
	Paths   *[]string
}

func (t *GetLogs) SetPaths(l *[]string) *GetLogs {
	t.Paths = l
	return t
}

func (t *GetLogs) SetFilters(filters *[]string) *GetLogs {
	t.Filters = filters
	return t
}

// NewGetLogs allocates a EventsCmdConfig struct and sets
// default values to its keys.
func NewGetLogs(t api.ClientInterface) *GetLogs {
	options := &GetLogs{
		client: t,
	}
	return options
}

// GetRaw fetchs an event json RawMessage stream from the agent api
func (t GetLogs) GetRaw() (chan []byte, error) {
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
func (t GetLogs) Do() (chan event.Event, error) {
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

// GetReader returns event.ReadCloser for GetLogReader
func (t *GetLogs) GetReader() (event.ReadCloser, error) {
	resp, err := t.eventsBase()
	if err != nil {
		return nil, err
	}
	return sseevent.NewReadCloser(resp.Body), nil
}

func (t GetLogs) eventsBase() (resp *http.Response, err error) {
	if t.Paths != nil {
		params := api.GetInstancesLogsParams{
			Filter: t.Filters,
			Paths:  *t.Paths,
		}
		resp, err = t.client.GetInstancesLogs(context.Background(), &params)
	} else {
		params := api.GetNodeLogsParams{
			Filter: t.Filters,
		}
		resp, err = t.client.GetNodeLogs(context.Background(), &params)
	}
	if err == nil && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected get events status code %s", resp.Status)
	}
	return resp, err
}
