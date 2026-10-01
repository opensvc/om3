package object

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/sanswitch"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// SwitchPush is what came of pushing one SAN switch to the collector.
	SwitchPush struct {
		Name string `json:"name"`
		Type string `json:"type"`

		// Commands is the number of switch commands whose output was
		// pushed.
		Commands int `json:"commands"`

		// Via says which collector api received the push: "oc3", or
		// "jsonrpc" where oc3 does not serve the switch feed.
		Via string `json:"via,omitempty"`

		Error string `json:"error,omitempty"`
	}

	// SwitchItem is a switch section of the node or cluster configuration.
	SwitchItem struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	oc3SANSwitchBody struct {
		Name string            `json:"name"`
		Type string            `json:"type"`
		Data map[string]string `json:"data"`
	}
)

var (
	// errFeedNotServed is an oc3 answering 404 to the switch feed: an oc3
	// older than it.
	errFeedNotServed = errors.New("the collector feeder does not serve this feed")

	// pushSwitchTimeout bounds the post of one switch, whose zoning can
	// weigh megabytes.
	pushSwitchTimeout = 30 * time.Second
)

// ListSwitches returns the switch sections of the node and cluster
// configuration.
func (t *Node) ListSwitches() []SwitchItem {
	l := make([]SwitchItem, 0)
	for _, s := range t.MergedConfig().SectionStrings() {
		if !strings.HasPrefix(s, "switch#") {
			continue
		}
		l = append(l, SwitchItem{
			Name: strings.TrimPrefix(s, "switch#"),
			Type: t.MergedConfig().Get(key.New(s, "type")),
		})
	}
	return l
}

// Switch returns the driver of the switch named in the node or cluster
// configuration, or nil when no section names it or no driver serves its
// type. The name is the one of the section, with or without its "switch#"
// prefix.
func (t *Node) Switch(name string) sanswitch.Driver {
	if !strings.HasPrefix(name, "switch#") {
		name = "switch#" + name
	}
	switchType := t.MergedConfig().Get(key.New(name, "type"))
	if switchType == "" {
		return nil
	}
	drv := sanswitch.GetDriver(switchType)
	if drv == nil {
		return nil
	}
	drv.SetName(name)
	drv.SetConfig(t.MergedConfig())
	return drv
}

// PushSwitches inventories the SAN switches of the node and cluster
// configuration, all of them or the one named, and reports their
// configuration to the collector.
//
// The report goes to the switch feed of oc3. Where oc3 does not serve it,
// because the node feeds the old collector only or because the oc3 it feeds
// is older than the feed, it goes to the "update_<type>" jsonrpc method of
// the old collector, as v2 sent it. That fallback is to be removed when oc3
// has replaced the old collector, which drops the jsonrpc api.
//
// One switch failing does not stop the others: a switch out of reach must
// not keep the reachable ones from being reported.
func (t Node) PushSwitches(ctx context.Context, name string) ([]SwitchPush, error) {
	name = strings.TrimPrefix(name, "switch#")
	l := make([]SwitchPush, 0)
	var errs error
	for _, item := range t.ListSwitches() {
		if name != "" && item.Name != name {
			continue
		}
		push := SwitchPush{Name: item.Name, Type: item.Type}
		if err := t.pushSwitch(ctx, item, &push); err != nil {
			push.Error = err.Error()
			t.Log().Attr("switch", item.Name).Warnf("push switch %s: %s", item.Name, err)
			errs = errors.Join(errs, fmt.Errorf("switch %s: %w", item.Name, err))
		}
		l = append(l, push)
	}
	if name != "" && len(l) == 0 {
		return l, fmt.Errorf("no switch found matching %s in the node or cluster config", name)
	}
	return l, errs
}

func (t Node) pushSwitch(ctx context.Context, item SwitchItem, push *SwitchPush) error {
	drv := t.Switch(item.Name)
	if drv == nil {
		return fmt.Errorf("no switch driver found matching type %s", item.Type)
	}
	data, err := drv.Report(ctx)
	if err != nil {
		return err
	}
	push.Commands = len(data)
	reportedName := drv.ReportName()
	err = t.postSANSwitch(ctx, reportedName, item.Type, data)
	switch {
	case err == nil:
		push.Via = "oc3"
		return nil
	case errors.Is(err, collector.ErrConfig), errors.Is(err, errFeedNotServed):
		t.Log().Attr("switch", item.Name).Infof("push switch %s through the jsonrpc api: %s", item.Name, err)
	default:
		return err
	}
	if err := t.callUpdateSANSwitch(reportedName, item.Type, data); err != nil {
		return err
	}
	push.Via = "jsonrpc"
	return nil
}

// postSANSwitch posts the switch report to the oc3 switch feed.
func (t Node) postSANSwitch(ctx context.Context, name, switchType string, data map[string]string) error {
	oc3, err := t.CollectorFeeder()
	if err != nil {
		return err
	}
	b, err := json.Marshal(oc3SANSwitchBody{Name: name, Type: switchType, Data: data})
	if err != nil {
		return fmt.Errorf("encode request body: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, pushSwitchTimeout)
	defer cancel()
	method, path := http.MethodPost, oc3path.FeedSANSwitch
	req, err := oc3.NewRequestWithContext(ctx, method, path, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("create collector request %s %s: %w", method, path, err)
	}
	resp, err := oc3.Do(req)
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusAccepted:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s %s", errFeedNotServed, method, path)
	default:
		return fmt.Errorf("unexpected collector response status code for %s %s: wanted %d got %d",
			method, path, http.StatusAccepted, resp.StatusCode)
	}
}

// callUpdateSANSwitch reports the switch through the jsonrpc method v2 used,
// "update_brocade" for a brocade, with the command outputs named as v2 named
// them, "brocadeswitchshow" and so on, which the old collector stores them
// under.
func (t Node) callUpdateSANSwitch(name, switchType string, data map[string]string) error {
	client, err := t.CollectorFeedClient()
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(data))
	values := make([]any, 0, len(data))
	for _, cmd := range sanSwitchCommandOrder(data) {
		keys = append(keys, switchType+cmd)
		values = append(values, data[cmd])
	}
	method := "update_" + switchType
	response, err := client.Call(method, name, keys, values)
	if err != nil {
		return err
	}
	if response != nil && response.Error != nil {
		return fmt.Errorf("%s: %s", method, response.Error.Message)
	}
	return nil
}

// sanSwitchCommandOrder returns the commands of a report in the order v2
// sent them, and any other after, sorted.
func sanSwitchCommandOrder(data map[string]string) []string {
	known := []string{"switchshow", "nsshow", "zoneshow"}
	l := make([]string, 0, len(data))
	for _, cmd := range known {
		if _, ok := data[cmd]; ok {
			l = append(l, cmd)
		}
	}
	others := make([]string, 0)
	for cmd := range data {
		if !slices.Contains(known, cmd) {
			others = append(others, cmd)
		}
	}
	sort.Strings(others)
	return append(l, others...)
}
