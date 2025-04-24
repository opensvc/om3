// Package commoncmd provides utilities and shared functionality to facilitate
// operations related to managing remotes objects, nodes, and logs for omcmd
// and oxcmd.
package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/xmap"
)

type (
	OptsGlobal struct {
		Color          string
		Output         string
		ObjectSelector string
	}
)

func MergeSelector(selector string, subsysSelector string, kind string, defaultSelector string) string {
	var s string
	switch {
	case selector != "":
		s = selector
	case subsysSelector != "":
		s = subsysSelector
	default:
		s = defaultSelector
	}
	if p, err := naming.ParsePath(s); err == nil {
		if err == nil {
			if kind == "" {
				return s
			}
			if p.Kind.String() == kind {
				return s
			}
		}
	}
	if (subsysSelector != "") && (kind != "") {
		kindSelector := "*/" + kind + "/*"
		if s == "" {
			s = kindSelector
		} else {
			s += "+" + kindSelector
		}
	}
	return s
}

func NodesFromPaths(c *client.T, selector string) ([]string, error) {
	m := make(map[string]any)
	params := api.GetObjectsParams{Path: &selector}
	resp, err := c.GetObjectsWithResponse(context.Background(), &params)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("%s", resp.Status())
	}
	for _, item := range resp.JSON200.Items {
		for node := range item.Data.Instances {
			m[node] = nil
		}
	}
	return xmap.Keys(m), nil
}

func AnySingleNode(selector string, c *client.T) (string, error) {
	if selector == "" {
		return "localhost", nil
	}
	nodenames, err := nodeselector.New(selector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return "", err
	}
	switch len(nodenames) {
	case 0:
		return "", fmt.Errorf("no matching node")
	case 1:
	default:
		return "", fmt.Errorf("more than one matching node: %s", nodenames)
	}
	return nodenames[0], nil
}

func WaitInstanceMonitor(ctx context.Context, c *client.T, p naming.Path, timeout time.Duration, errC chan error) error {
	var (
		err      error
		evReader event.ReadCloser
	)
	filters := []string{"InstanceMonitorUpdated,path=" + p.String()}
	getEvents := c.NewGetEvents().SetFilters(filters).SetLimit(1)
	if timeout > 0 {
		getEvents = getEvents.SetDuration(timeout)
	}
	evReader, err = getEvents.GetReader()
	if err != nil {
		return err
	}

	if x, ok := evReader.(event.ContextSetter); ok {
		x.SetContext(ctx)
	}
	go func() {
		defer func() {
			if err != nil {
				err = fmt.Errorf("wait instance monitor update failed on object %s: %w", p, err)
			}
			select {
			case <-ctx.Done():
			case errC <- err:
			}
		}()

		go func() {
			// close reader when ctx is done
			select {
			case <-ctx.Done():
				_ = evReader.Close()
			}
		}()
		for {
			ev, readError := evReader.Read()
			if readError != nil {
				if errors.Is(readError, io.EOF) {
					err = fmt.Errorf("no more events, wait %v failed %s: %w", p, time.Now(), err)
				} else {
					err = readError
				}
				return
			}
			_, err = msgbus.EventToMessage(*ev)
			return
		}
	}()
	return nil
}
