package daemonapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/event/sseevent"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
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

		// DataFilters is a slice of DataFilter used to define filtering data conditions
		// based on key, value, and operator.
		DataFilters DataFilters
	}

	// DataFilter represents a filtering data condition based on a key, value,
	// and an operator.
	DataFilter struct {
		Key   string
		Value string
		Op    string
	}

	// DataFilters is a slice of DataFilter used to define a collection of filtering data conditions.
	DataFilters []DataFilter

	kinder interface {
		Kind() string
	}
)

// GetDaemonEvents feeds node daemon event publications in rss format.
func (a *DaemonAPI) GetDaemonEvents(ctx echo.Context, nodename string, params api.GetDaemonEventsParams) error {
	if nodename == a.localhost || nodename == "localhost" {
		return a.getLocalDaemonEvents(ctx, params)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	} else {
		return a.getPeerDaemonEvents(ctx, nodename, params)
	}
}

func (a *DaemonAPI) getPeerDaemonEvents(ctx echo.Context, nodename string, params api.GetDaemonEventsParams) error {
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
	c, err := a.newProxyClient(ctx, nodename, clientOptions...)
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

// getLocalDaemonEvents handles streaming local daemon events based on provided filters, selectors, and parameters.
func (a *DaemonAPI) getLocalDaemonEvents(ctx echo.Context, params api.GetDaemonEventsParams) error {
	if v, err := assertRole(ctx, rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin, rbac.RoleRoot, rbac.RoleJoin, rbac.RoleLeave); !v {
		return err
	}
	var (
		handlerName = "GetDaemonEvents"
		limit       uint64
		eventCount  uint64
		err         error

		// hasSelector is true when param.Selector is defined and not ""
		hasSelector bool

		// pathL list of all cluster object paths
		pathL naming.Paths

		// pathM all cluster object paths
		pathM naming.M

		// selector is used to expand param.Selector vs pathL
		selector *objectselector.Selection

		// pathSelected is a map of currently selected object paths
		pathSelected naming.M

		// requestedFilterByFilterIdentifier is a map indexed on requested
		// filter identifiers.
		// It is used to differentiate the requested filters from extra filters
		// added for object selection but not for response event stream.
		requestedFilterByFilterIdentifier = make(map[string]any)

		// dataFiltersByKind is a map of DataFilters indexed on event kind.
		dataFiltersByKind = make(map[string]DataFilters)

		evCtx  = ctx.Request().Context()
		cancel context.CancelFunc
	)
	hasRoot := grantsFromContext(ctx).HasRole(rbac.RoleRoot)
	userGrants := grantsFromContext(ctx)

	log := LogHandler(ctx, handlerName)
	log.Debugf("starting")
	defer log.Debugf("done")

	getSelectedMap := func() (naming.M, error) {
		if selected, err := selector.Expand(); err != nil {
			return nil, err
		} else {
			return selected.StrMap(), nil
		}
	}

	// needForwardEvent returns true when event needs to be forwarded to
	// response event stream because it matches one of param.Filters
	needForwardEvent := func(kind string, m pubsub.Messager) bool {
		labels := m.GetLabels()
		for _, k := range labels.Keys() {
			// need verify both kind:label and nil:label
			for _, s := range []string{kind + ":" + k, kind + ":" + k} {
				if _, ok := requestedFilterByFilterIdentifier[s]; ok {
					return true
				}
			}
		}
		return false
	}

	// isAllowed returns false if a message has a namespace label that
	// doesn't match any of the user's guest grant.
	isAllowed := func(msg pubsub.Messager) bool {
		if hasRoot {
			return true
		}
		labels := msg.GetLabels()
		if namespace, ok := labels["namespace"]; ok {
			return userGrants.Has(rbac.RoleGuest, namespace)
		}
		return true
	}

	// isSelected returns true when msg has path label that is selected or
	// doesn't have a path label.
	isSelected := func(msg pubsub.Messager) bool {
		labels := msg.GetLabels()
		if s, ok := labels["path"]; ok {
			if pathSelected.Has(s) {
				// path label is selected
				return true
			}
			// path label is not selected
			return false
		}
		// no path label
		return true
	}

	if params.Selector != nil && *params.Selector != "" {
		hasSelector = true
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

	name := fmt.Sprintf("api.get_daemon_event from %s %s", ctx.Request().RemoteAddr, ctx.Get("uuid"))
	if params.Filter != nil && len(*params.Filter) > 0 {
		name += " filters: [" + strings.Join(*params.Filter, " ") + "]"
	}

	a.announceSub(name)
	defer a.announceUnsub(name)

	sub := a.SubFactory.Sub(name, pubsub.Timeout(time.Second), a.SubQS)

	for _, filter := range filters {
		if filter.Kind == nil {
			log.Debugf("filtering %v %v", filter.Kind, filter.Labels)
			requestedFilterByFilterIdentifier[pubsub.FilterFmt("", filter.Labels...)] = nil
		} else if kind, ok := filter.Kind.(event.Kinder); ok {
			requestedFilterByFilterIdentifier[pubsub.FilterFmt(kind.Kind(), filter.Labels...)] = nil

			if len(filter.DataFilters) > 0 {
				log.Debugf("filtering %s label:%v data:%v", kind.Kind(), filter.Labels, filter.DataFilters)
				dataFiltersByKind[kind.Kind()] = filter.DataFilters
			} else {
				log.Debugf("filtering %s label:%v", kind.Kind(), filter.Labels)
				log.Debugf("filtering %s %v", kind.Kind(), filter.Labels)
			}
		} else {
			log.Warnf("skip filtering of %s %v", reflect.TypeOf(filter.Kind), filter.Labels)
			continue
		}
		sub.AddFilter(filter.Kind, filter.Labels...)
	}
	if hasSelector && len(requestedFilterByFilterIdentifier) == 0 {
		// no filters => all events must be forwarded, add ObjectCreated &
		// ObjectDeleted to requestedFilterByFilterIdentifier to simulate client has asked for them
		requestedFilterByFilterIdentifier["ObjectCreated:"] = nil
		requestedFilterByFilterIdentifier["ObjectDeleted:{node="+a.localhost+"}"] = nil
	}

	if hasSelector {
		// when request filters don't match ObjectCreated or
		// ObjectDeleted,node=<localhost>. We create "hidden" filter for them.
		// "hidden" because such messages don't require to be forwarded
		// to response event stream.
		createdMsg := &msgbus.ObjectCreated{}
		createdMsg.AddLabels(a.LabelLocalhost)
		if !needForwardEvent("ObjectCreated", createdMsg) {
			log.Debugf("add hidden filtering: ObjectCreated")
			sub.AddFilter(&msgbus.ObjectCreated{})
		}
		deleteMsg := &msgbus.ObjectDeleted{}
		deleteMsg.AddLabels(a.LabelLocalhost)
		if !needForwardEvent("ObjectDeleted", deleteMsg) {
			log.Debugf("add hidden filtering: ObjectDeleted,node=%s", a.localhost)
			sub.AddFilter(&msgbus.ObjectDeleted{}, a.LabelLocalhost)
		}
	}
	sub.Start()
	defer func() {
		if err := sub.Stop(); err != nil {
			log.Debugf("sub.Stop: %s", err)
		}
	}()
	if hasSelector {
		pathL = object.StatusData.GetPaths()
		pathM = pathL.StrMap()
		selector = objectselector.New(
			*params.Selector,
			objectselector.WithPaths(pathL),
			objectselector.WithLocal(true),
			objectselector.WithConfigFilterDisabled(),
		)
		if err := selector.CheckFilters(); err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest,
				"Invalid filters", "%s", err)
		}
		if selected, err := getSelectedMap(); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError,
				"get object paths", "from selector %s: %s",
				*params.Selector, err)
		} else {
			pathSelected = selected
		}
	}
	w.WriteHeader(http.StatusOK)

	// don't wait first event to flush response
	w.Flush()

	sseWriter := sseevent.NewWriter(w)
	evCounter := uint64(0)

	doEvent := func(i any) error {
		ev := event.ToEvent(i, evCounter)
		if ev != nil {
			if dataFilter, ok := dataFiltersByKind[ev.Kind]; ok {
				v := make(map[string]any)
				if err := json.Unmarshal(ev.Data, &v); err != nil {
					return err
				}
				if !dataFilter.match(v) {
					return nil
				}
			}
		}
		evCounter++
		if _, err := sseWriter.Write(ev); err != nil {
			log.Debugf("write event %s: %s", ev.Kind, err)
			return err
		}
		w.Flush()
		return nil
	}

	if params.Cache != nil && *params.Cache {
		data := msgbus.NewClusterData(a.Daemondata.ClusterData())
		for _, filter := range filters {
			labels := pubsub.Labels{}
			for _, lab := range filter.Labels {
				labels[lab[0]] = lab[1]
			}
			anyL, err := data.ExtractEvents(filter.Kind, labels)
			if err != nil {
				return fmt.Errorf("get cached data: %w", err)
			}
			for _, anyE := range anyL {
				if err := doEvent(anyE); err != nil {
					log.Debugf("do event failed on %v: %s", anyE, err)
					return nil
				}
				w.Flush()
				if limit > 0 && eventCount >= limit {
					return nil
				}
			}
		}
	}

	for {
		select {
		case <-evCtx.Done():
			return nil
		case i := <-sub.C:
			if ev, ok := i.(pubsub.Messager); ok {
				if !isAllowed(ev) {
					continue
				}
			}
			if hasSelector {
				switch ev := i.(type) {
				case *msgbus.ObjectCreated:
					s := ev.Path.String()
					if !pathM.Has(s) {
						pathL = pathL.Merge([]naming.Path{ev.Path})
						pathM[s] = nil
						selector.SetPaths(pathL)
						if selected, err := getSelectedMap(); err != nil {
							log.Errorf("can't filter on object created")
							return err
						} else if selected.Has(s) {
							log.Debugf("add created object %s to selection", s)
							pathSelected[s] = nil
						}
					}
					if !needForwardEvent("ObjectCreated", ev) {
						// not required on response stream
						continue
					}
					if !isSelected(ev) {
						// message is not for selected path
						continue
					}
					// message will be forwarded
				case *msgbus.ObjectDeleted:
					notAnymoreSelected := false
					if ev.GetLabels()["node"] == a.localhost {
						s := ev.Path.String()
						if pathSelected.Has(s) {
							notAnymoreSelected = true
							log.Debugf("remove deleted object %s from selection", s)
							delete(pathSelected, s)
						}
						if _, ok := pathM[s]; ok {
							delete(pathM, s)
							// TODO implement naming.Paths.Drop(p naming.Path)
							newPathL := make(naming.Paths, 0)
							for _, p := range pathL {
								if p.Equal(ev.Path) {
									continue
								}
								newPathL = append(newPathL, p)
							}
							pathL = newPathL
						}
					}
					if !needForwardEvent("ObjectDeleted", ev) {
						// not required on response stream
						continue
					}
					if notAnymoreSelected {
						// message from a previously selected path, that will
						// be now discarted, we have to send this last message
					} else if !isSelected(ev) {
						// message is not for selected path
						continue
					}
					// message will be forwarded
				case pubsub.Messager:
					if !isSelected(ev) {
						// message is not for selected path
						continue
					}
					// message will be forwarded
				}
			}

			if err := doEvent(i); err != nil {
				log.Warnf("doEvent error for %v: %s", i, err)
				return nil
			}
			if limit > 0 && eventCount >= limit {
				return nil
			}
		}
	}
}

// parseFilters return filters from b.Filter
func parseFilters(params api.GetDaemonEventsParams) (filters []Filter, err error) {
	var filter Filter
	matchKind := make(map[string]bool)

	if params.Filter == nil {
		return
	}

	for _, s := range *params.Filter {
		if len(s) == 0 {
			continue
		}
		filter, err = parseFilter(s)
		if err != nil {
			return
		}
		if filter.IsZero() {
			continue
		}
		if k, ok := filter.Kind.(kinder); ok {
			kind := k.Kind()
			hasMatcher, alreadyFiltered := matchKind[kind]
			if hasMatcher || (alreadyFiltered && len(filter.DataFilters) > 0) {
				return nil, fmt.Errorf("can't filter same kind multiple times when it has a value matcher: %s", kind)
			}
			matchKind[kind] = len(filter.DataFilters) > 0
		}
		filters = append(filters, filter)
	}
	return
}

// parseFilter return filter from s
//
// filter syntax is: [kind][,label=value][,.abcd.efgh=value]*
func parseFilter(filterStr string) (Filter, error) {
	var filter Filter

	isAlphanumeric := func(s string) bool {
		for _, r := range s {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				return false
			}
		}
		return true
	}

	parseKind := func(s string) (rest string, kind any, err error) {
		if i := strings.Index(s, ","); i < 0 {
			if isAlphanumeric(s) {
				// <kind> => match the specified kind
				rest = ""
				kind, err = msgbus.KindToT(s)
			} else {
				// <filter>,... => match all kinds
				rest = s
				kind = nil
			}
		} else if i == 0 {
			// ,<filter>,... => match all kinds
			rest = s[1:]
			kind = nil
		} else {
			// <kind>,<filter>,... => match the specified kind
			rest = s[i+1:]
			kind, err = msgbus.KindToT(s[0:i])
		}
		return
	}

	if rest, kind, err := parseKind(filterStr); err != nil {
		return Filter{}, err
	} else {
		filter.Kind = kind
		filterStr = rest
	}

	if len(filterStr) == 0 {
		// no label filters
		return filter, nil
	}

	parseLabelFilter := func(s string) (pubsub.Label, error) {
		split := strings.SplitN(s, "=", 2)
		if len(split) != 2 {
			return pubsub.Label{}, fmt.Errorf("invalid label filter expression: %s (expecting <key><op><value>)", filterStr)
		}
		key := strings.TrimSpace(split[0])
		value := strings.TrimSpace(split[1])
		if len(key) == 0 {
			return pubsub.Label{}, fmt.Errorf("invalid label filter expression: %s (empty key)", filterStr)
		}
		return pubsub.Label{key, value}, nil
	}

	parseDataFilter := func(s string) (DataFilter, error) {
		ops := []string{
			"!=", ">=", "<=", // keep longer operators first
			"=", ">", "<",
		}
		for _, op := range ops {
			split := strings.SplitN(s, op, 2)
			if len(split) != 2 {
				continue
			}
			key := strings.TrimSpace(split[0])
			value := strings.TrimSpace(split[1])
			return DataFilter{Key: key, Value: value, Op: op}, nil
		}
		return DataFilter{}, fmt.Errorf("invalid filter expression: %s (unknown operator)", s)
	}

	for _, filterElement := range strings.Split(filterStr, ",") {
		switch {
		case len(filterElement) == 0:
			continue
		case strings.HasPrefix(filterElement, "."):
			dataFilter, err := parseDataFilter(filterElement)
			if err != nil {
				return filter, err
			}
			filter.DataFilters = append(filter.DataFilters, dataFilter)
		default:
			label, err := parseLabelFilter(filterElement)
			if err != nil {
				return filter, err
			}
			filter.Labels = append(filter.Labels, label)
		}
	}
	return filter, nil
}

func (f Filter) IsZero() bool {
	return f.Kind == nil && len(f.DataFilters) == 0 && len(f.Labels) == 0
}

func (df DataFilters) match(i any) bool {
	flatten := output.Flatten(i)

	intLessOrEqual := func(str1, str2 string) (bool, error) {
		num1, err1 := strconv.Atoi(str1)
		num2, err2 := strconv.Atoi(str2)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("invalid input: both strings must be valid integers")
		}
		return num1 <= num2, nil
	}

	intLess := func(str1, str2 string) (bool, error) {
		num1, err1 := strconv.Atoi(str1)
		num2, err2 := strconv.Atoi(str2)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("invalid input: both strings must be valid integers")
		}
		return num1 < num2, nil
	}

	intGreaterOrEqual := func(str1, str2 string) (bool, error) {
		num1, err1 := strconv.Atoi(str1)
		num2, err2 := strconv.Atoi(str2)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("invalid input: both strings must be valid integers")
		}
		return num1 >= num2, nil
	}

	intGreater := func(str1, str2 string) (bool, error) {
		num1, err1 := strconv.Atoi(str1)
		num2, err2 := strconv.Atoi(str2)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("invalid input: both strings must be valid integers")
		}
		return num1 > num2, nil
	}

	float64LessOrEqual := func(str1, str2 string) (bool, error) {
		num1, err1 := strconv.ParseFloat(str1, 64)
		num2, err2 := strconv.ParseFloat(str2, 64)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("invalid input: both strings must be valid float64")
		}
		return num1 <= num2, nil
	}

	float64Less := func(str1, str2 string) (bool, error) {
		num1, err1 := strconv.ParseFloat(str1, 64)
		num2, err2 := strconv.ParseFloat(str2, 64)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("invalid input: both strings must be valid float64")
		}
		return num1 < num2, nil
	}

	float64GreaterOrEqual := func(str1, str2 string) (bool, error) {
		num1, err1 := strconv.ParseFloat(str1, 64)
		num2, err2 := strconv.ParseFloat(str2, 64)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("invalid input: both strings must be valid float64")
		}
		return num1 >= num2, nil
	}

	float64Greater := func(str1, str2 string) (bool, error) {
		num1, err1 := strconv.ParseFloat(str1, 64)
		num2, err2 := strconv.ParseFloat(str2, 64)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("invalid input: both strings must be valid float64")
		}
		return num1 > num2, nil
	}

	matchDataFilter := func(m DataFilter) bool {
		s, ok := flatten[m.Key]
		if !ok {
			return false
		}
		s = strings.TrimSpace(s)
		switch m.Op {
		case "=":
			return s == m.Value
		case "!=":
			return s != m.Value
		case "<":
			if v, err := intLess(s, m.Value); err == nil {
				return v
			}
			if v, err := float64Less(s, m.Value); err == nil {
				return v
			}
			return s < m.Value
		case ">":
			if v, err := intGreater(s, m.Value); err == nil {
				return v
			}
			if v, err := float64Greater(s, m.Value); err == nil {
				return v
			}
			return s > m.Value
		case "<=":
			if v, err := intLessOrEqual(s, m.Value); err == nil {
				return v
			}
			if v, err := float64LessOrEqual(s, m.Value); err == nil {
				return v
			}
			return s <= m.Value
		case ">=":
			if v, err := intGreaterOrEqual(s, m.Value); err == nil {
				return v
			}
			if v, err := float64GreaterOrEqual(s, m.Value); err == nil {
				return v
			}
			return s >= m.Value
		default:
			return false
		}
	}

	for _, m := range df {
		if !matchDataFilter(m) {
			return false
		}
	}
	return true
}
