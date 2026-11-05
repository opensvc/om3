package object

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

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

// pushSwitchTimeout bounds the post of one switch, whose zoning can weigh
// megabytes.
var pushSwitchTimeout = 30 * time.Second

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
// The report goes to the switch feed of the oc3 feeder.
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
	return t.postSANSwitch(ctx, drv.ReportName(), item.Type, data)
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
	return CollectorResponseStatusCheck(resp, method, path, []int{http.StatusAccepted})
}
