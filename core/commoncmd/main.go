// Package commoncmd provides utilities and shared functionality to facilitate
// operations related to managing remotes objects, nodes, and logs for omcmd
// and oxcmd.
package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/xmap"
	"github.com/opensvc/om3/util/xsession"
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
		core, err := item.Data.AsObjectCore()
		if err != nil {
			return []string{}, err
		}
		for node := range core.Instances {
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

// WaitAllInstanceMonitor launches a go routine that waits for all instance monitor
// updated events are received for an object within a given timeout, or ctx.Done() reached.
// The result of the wait is sent to the errC channel: nil if the
// InstanceMonitorUpdated event occurs, or a non-nil error otherwise.
//
// It also filters the ObjectConfigUpdated events to get the expected Instance monitor
// updated events from the instance config scope nodes.
//
// If acquiring the event reader fails, WaitInstanceMonitor returns
// the error immediately and does not send anything to errC.
func WaitAllInstanceMonitor(ctx context.Context, c *client.T, p naming.Path, timeout time.Duration, errC chan error) error {
	waitingAt := time.Now()
	filters := []string{
		"InstanceMonitorUpdated,path=" + p.String(),
		"InstanceConfigUpdated,path=" + p.String(),
	}
	getEvents := c.NewGetEvents().SetFilters(filters)
	if timeout > 0 {
		getEvents = getEvents.SetDuration(timeout)
	}
	evReader, err := getEvents.GetReader()
	if err != nil {
		return err
	}

	var closeOnce sync.Once
	closeReader := func() {
		closeOnce.Do(func() {
			_ = evReader.Close()
		})
	}

	if x, ok := evReader.(event.ContextSetter); ok {
		x.SetContext(ctx)
	} else {
		// evReader is not an event.ContextSetter, we have to early closeReader
		// to early stop evReader.Read() within the wait event routine.
		go func() {
			select {
			case <-ctx.Done():
				closeReader()
			}
		}()
	}
	go func() {
		defer closeReader()
		var msgEvent any
		monM := make(map[string]struct{})
		nodeM := make(map[string]struct{})
		for {
			rawEvent, err := evReader.Read()
			if err != nil {
				if errors.Is(err, io.EOF) {
					err = fmt.Errorf("no more events: %w", err)
				}
			} else {
				msgEvent, err = msgbus.EventToMessage(*rawEvent)
			}
			if err != nil {
				elapsed := time.Now().Sub(waitingAt)
				err = fmt.Errorf("wait instance monitor update failed on object %s after %s: %w", p, elapsed, err)
				errC <- err
				break
			}
			switch ev := msgEvent.(type) {
			case *msgbus.InstanceConfigUpdated:
				for _, n := range ev.Value.Scope {
					nodeM[n] = struct{}{}
				}
			case *msgbus.InstanceMonitorUpdated:
				monM[ev.Node] = struct{}{}
			}
			if len(monM) == len(nodeM) {
				errC <- nil
				break
			}
		}
		return
	}()
	return nil
}

// WaitInstanceMonitor launches a go routine that waits for a specific
// instance monitor update event within a given timeout, or ctx.Done() reached.
// The result of the wait is sent to the errC channel: nil if the
// InstanceMonitorUpdated event occurs, or a non-nil error otherwise.
//
// If acquiring the event reader fails, WaitInstanceMonitor returns
// the error immediately and does not send anything to errC.
func WaitInstanceMonitor(ctx context.Context, c *client.T, p naming.Path, timeout time.Duration, errC chan error) error {
	waitingAt := time.Now()
	filters := []string{"InstanceMonitorUpdated,path=" + p.String()}
	getEvents := c.NewGetEvents().SetFilters(filters).SetLimit(1)
	if timeout > 0 {
		getEvents = getEvents.SetDuration(timeout)
	}
	evReader, err := getEvents.GetReader()
	if err != nil {
		return err
	}

	var closeOnce sync.Once
	closeReader := func() {
		closeOnce.Do(func() {
			_ = evReader.Close()
		})
	}

	if x, ok := evReader.(event.ContextSetter); ok {
		x.SetContext(ctx)
	} else {
		// evReader is not an event.ContextSetter, we have to early closeReader
		// to early stop evReader.Read() within the wait event routine.
		go func() {
			select {
			case <-ctx.Done():
				closeReader()
			}
		}()
	}
	go func() {
		defer closeReader()
		ev, err := evReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = fmt.Errorf("no more events: %w", err)
			}
		} else {
			_, err = msgbus.EventToMessage(*ev)
		}
		if err != nil {
			elapsed := time.Now().Sub(waitingAt)
			err = fmt.Errorf("wait instance monitor update failed on object %s after %s: %w", p, elapsed, err)
		}
		errC <- err
		return
	}()
	return nil
}

// WaitInstanceStatusUpdated launches a go routine that waits for a specific
// instance status update event within a given timeout, or ctx.Done() reached.
// The result of the wait is sent to the errC channel: nil if the
// InstanceStatusUpdated event occurs, or a non-nil error otherwise.
//
// If acquiring the event reader fails, WaitInstanceStatusUpdated returns
// the error immediately and does not send anything to errC.
func WaitInstanceStatusUpdated(ctx context.Context, c *client.T, nodename string, p naming.Path, timeout time.Duration, errC chan error) error {
	waitingAt := time.Now()
	filters := []string{
		fmt.Sprintf("InstanceStatusUpdated,path=%s,node=%s", p.String(), nodename),
	}
	getEvents := c.NewGetEvents().SetFilters(filters).SetLimit(1)
	if timeout > 0 {
		getEvents = getEvents.SetDuration(timeout)
	}
	evReader, err := getEvents.GetReader()
	if err != nil {
		return err
	}

	var closeOnce sync.Once
	closeReader := func() {
		closeOnce.Do(func() {
			_ = evReader.Close()
		})
	}

	if x, ok := evReader.(event.ContextSetter); ok {
		x.SetContext(ctx)
	} else {
		// evReader is not an event.ContextSetter, we have to early closeReader
		// to early stop evReader.Read() within the wait event routine.
		go func() {
			select {
			case <-ctx.Done():
				closeReader()
			}
		}()
	}

	go func() {
		defer closeReader()
		ev, err := evReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = fmt.Errorf("no more events: %w", err)
			}
		} else {
			_, err = msgbus.EventToMessage(*ev)
		}
		if err != nil {
			elapsed := time.Now().Sub(waitingAt)
			err = fmt.Errorf("wait instance status update failed on %s@%s after %s: %w", p, nodename, elapsed, err)
		}
		errC <- err
	}()
	return nil
}

// RefreshInstanceStatusFromClusterStatus updates instance statuses for all nodes using cluster status data.
// It ensures instance status updates before return.
// Returns an error if client creation, API calls, or processing fails.
func RefreshInstanceStatusFromClusterStatus(ctx context.Context, clusterStatus clusterdump.Data) error {
	var wg sync.WaitGroup
	sid := api.InQueryRequesterSid(xsession.ID)
	params := &api.PostInstanceActionStatusParams{
		RequesterSid: &sid,
	}
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}

	// serialize the WaitInstanceStatusUpdated calls and go wait for its
	// completion: The WaitInstanceStatusUpdated have to be called before refresh
	// calls.
	for nodename, node := range clusterStatus.Cluster.Node {
		for ps, _ := range node.Instance {
			path, _ := naming.ParsePath(ps)

			// errC must be buffered because of early return if an error occurs
			// during PostInstanceActionStatusWithResponse
			errC := make(chan error, 1)

			if err := WaitInstanceStatusUpdated(ctx, c, nodename, path, 0, errC); err != nil {
				// TODO: accumulate or ignore error ?
			} else {
				wg.Add(1)
				go func(c <-chan error) {
					_ = <-c
					wg.Done()
				}(errC)
			}
		}
	}
	for nodename, node := range clusterStatus.Cluster.Node {
		for ps, _ := range node.Instance {
			path, _ := naming.ParsePath(ps)
			response, err := c.PostInstanceActionStatusWithResponse(ctx, nodename, path.Namespace, path.Kind, path.Name, params)
			if err != nil {
				return err
			}
			switch response.StatusCode() {
			case 200:
			default:
				return fmt.Errorf("%s: %s: post status refresh: %d", nodename, path, response.StatusCode())
			}
		}
	}
	wg.Wait()
	return nil
}
