package daemonapi

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/event/sseevent"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	Filter struct {
		Kind   any
		Labels []pubsub.Label
	}
)

// GetDaemonEvents feeds node daemon event publications in rss format.
func (a *DaemonApi) GetDaemonEvents(ctx echo.Context, nodename string, params api.GetDaemonEventsParams) error {
	if nodename == a.localhost || nodename == "localhost" {
		return a.getLocalDaemonEvents(ctx, params)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	} else {
		return a.getPeerDaemonEvents(ctx, nodename, params)
	}
}

func (a *DaemonApi) getPeerDaemonEvents(ctx echo.Context, nodename string, params api.GetDaemonEventsParams) error {
	var (
		handlerName   = "getPeerDaemonEvents"
		limit         uint64
		eventCount    uint64
		clientOptions []funcopt.O
	)
	log := LogHandler(ctx, handlerName)
	evCtx := ctx.Request().Context()
	request := ctx.Request()
	if params.Duration != nil {
		if v, err := converters.Duration.Convert(*params.Duration); err != nil {
			log.Infof("Invalid parameter: field 'duration' with value '%s' validation error: %s", *params.Duration, err)
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'duration' with value '%s' validation error: %s", *params.Duration, err)
		} else if timeout := *v.(*time.Duration); timeout > 0 {
			var cancel context.CancelFunc
			evCtx, cancel = context.WithTimeout(evCtx, timeout)
			defer cancel()
			clientOptions = append(clientOptions, client.WithTimeout(timeout))
		} else {
			clientOptions = append(clientOptions, client.WithTimeout(0))
		}
	}
	if params.Limit != nil {
		limit = uint64(*params.Limit)
	}
	c, err := newProxyClient(ctx, nodename, clientOptions...)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}

	resp, err := c.GetDaemonEvents(evCtx, nodename, &params)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if resp.StatusCode != http.StatusOK {
		return JSONProblemf(ctx, resp.StatusCode, "Request peer", "%s: %s", nodename, err)
	}
	w := ctx.Response()
	if request.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	w.WriteHeader(http.StatusOK)

	// don't wait first event to flush response
	w.Flush()

	sseWriter := sseevent.NewWriter(w)
	evReader := sseevent.NewReadCloser(resp.Body)
	defer func() {
		if err := evReader.Close(); err != nil {
			log.Errorf("event reader close: %s", err)
		}
	}()
	for {
		ev, err := evReader.Read()
		if err != nil {
			log.Debugf("event read: %s", err)
			return nil
		} else if ev == nil {
			return nil
		}
		eventCount++
		if _, err := sseWriter.Write(ev); err != nil {
			log.Debugf("event write: %s", err)
			return nil
		}
		w.Flush()
		if limit > 0 && eventCount >= limit {
			log.Debugf("reach event count limit")
			return nil
		}
	}
}

// getLocalDaemonEvents feeds publications in rss format.
// TODO: Honor subscribers params.
func (a *DaemonApi) getLocalDaemonEvents(ctx echo.Context, params api.GetDaemonEventsParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot, rbac.RoleJoin); err != nil {
		return err
	} else if !v {
		return nil
	}
	var (
		handlerName = "GetDaemonEvents"
		limit       uint64
		eventCount  uint64
		pathMap     naming.M
		err         error

		evCtx  = ctx.Request().Context()
		cancel context.CancelFunc
	)
	log := LogHandler(ctx, handlerName)
	log.Debugf("starting")
	defer log.Debugf("done")

	getPathMap := func() (naming.M, error) {
		if params.Selector == nil {
			return nil, nil
		}
		paths, err := objectselector.NewSelection(
			*params.Selector,
			objectselector.SelectionWithInstalled(object.StatusData.GetPaths()),
			objectselector.SelectionWithLocal(true),
		).Expand()
		if err != nil {
			return nil, err
		}
		return paths.StrMap(), nil
	}

	if pathMap, err = getPathMap(); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Object selector", "%s", err)
	}
	if params.Limit != nil {
		limit = uint64(*params.Limit)
	}
	if params.Duration != nil {
		if v, err := converters.Duration.Convert(*params.Duration); err != nil {
			log.Infof("Invalid parameter: field 'duration' with value '%s' validation error: %s", *params.Duration, err)
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'duration' with value '%s' validation error: %s", *params.Duration, err)
		} else if timeout := *v.(*time.Duration); timeout > 0 {
			evCtx, cancel = context.WithTimeout(evCtx, timeout)
			defer cancel()
		}
	}

	filters, err := parseFilters(params)
	if err != nil {
		log.Infof("Invalid parameter: field 'filter' with value '%s' validation error: %s", *params.Filter, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'filter' with value '%s' validation error: %s", *params.Filter, err)
	}

	r := ctx.Request()
	w := ctx.Response()
	if r.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	name := fmt.Sprintf("lsnr-handler-event %s from %s %s", handlerName, ctx.Request().RemoteAddr, ctx.Get("uuid"))
	if params.Filter != nil && len(*params.Filter) > 0 {
		name += " filters: [" + strings.Join(*params.Filter, " ") + "]"
	}

	a.announceSub(name)
	defer a.announceUnsub(name)

	// objectSelectionSub is a subscription dedicated to object create/delete events.
	objectSelectionSub := a.EventBus.Sub(name, pubsub.Timeout(time.Second))
	objectSelectionSub.AddFilter(&msgbus.ObjectCreated{})
	objectSelectionSub.AddFilter(&msgbus.ObjectDeleted{})
	objectSelectionSub.Start()
	defer func() {
		if err := objectSelectionSub.Stop(); err != nil {
			log.Debugf("objectSelectionSub.Stop: %s", err)
		}
	}()

	sub := a.EventBus.Sub(name, pubsub.Timeout(time.Second))

	for _, filter := range filters {
		if filter.Kind == nil {
			log.Debugf("filtering %v %v", filter.Kind, filter.Labels)
		} else if kind, ok := filter.Kind.(event.Kinder); ok {
			log.Debugf("filtering %s %v", kind.Kind(), filter.Labels)
		} else {
			log.Warnf("skip filtering of %s %v", reflect.TypeOf(filter.Kind), filter.Labels)
			continue
		}
		sub.AddFilter(filter.Kind, filter.Labels...)
	}

	sub.Start()
	defer func() {
		if err := sub.Stop(); err != nil {
			log.Debugf("sub.Stop: %s", err)
		}
	}()

	w.WriteHeader(http.StatusOK)

	// don't wait first event to flush response
	w.Flush()

	sseWriter := sseevent.NewWriter(w)
	evCounter := uint64(0)
	for {
		select {
		case <-evCtx.Done():
			return nil
		case i := <-objectSelectionSub.C:
			if pathMap != nil {
				switch ev := i.(type) {
				case *msgbus.ObjectCreated:
					log.Infof("add created object %s to selection", ev.Path)
					pathMap[ev.Path.String()] = nil
				case *msgbus.ObjectDeleted:
					log.Infof("remove deleted object %s from selection", ev.Path)
					delete(pathMap, ev.Path.String())
				}
			}
		case i := <-sub.C:
			if pathMap != nil {
				// discard events with path label not matching the selector.
				msg := i.(pubsub.Messager)
				labels := msg.GetLabels()
				if s, ok := labels["path"]; ok && !pathMap.Has(s) {
					continue
				}
			}
			ev := event.ToEvent(i, evCounter)
			evCounter++
			if _, err := sseWriter.Write(ev); err != nil {
				log.Debugf("write event %s: %s", ev.Kind, err)
				return nil
			}
			w.Flush()
			if limit > 0 && eventCount >= limit {
				return nil
			}
		}
	}
}

// parseFilters return filters from b.Filter
func parseFilters(params api.GetDaemonEventsParams) (filters []Filter, err error) {
	var filter Filter

	if params.Filter == nil {
		return
	}

	for _, s := range *params.Filter {
		filter, err = parseFilter(s)
		if err != nil {
			return
		}
		filters = append(filters, filter)
	}
	return
}

// parseFilter return filter from s
//
// filter syntax is: [kind][,label=value]*
func parseFilter(s string) (filter Filter, err error) {
	for _, elem := range strings.Split(s, ",") {
		if strings.HasPrefix(elem, ".") {
			// TODO filter data ?
			continue
		}
		splitted := strings.SplitN(elem, "=", 2)
		if len(splitted) == 1 {
			// ignore error => use kind nil when value has invalid kind
			filter.Kind, _ = msgbus.KindToT(splitted[0])
		} else if len(splitted) == 2 {
			filter.Labels = append(filter.Labels, pubsub.Label{splitted[0], splitted[1]})
		} else {
			err = fmt.Errorf("invalid filter expression: %s", s)
			return
		}
	}
	return
}
