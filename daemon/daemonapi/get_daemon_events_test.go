package daemonapi

import (
	"fmt"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func TestGetDaemonEventsParamsOk(t *testing.T) {
	cases := map[string]struct {
		filterS  []string
		expected []Filter
	}{
		"type and label": {
			filterS: []string{"ObjectStatusUpdated,path=root/svc/foo"},
			expected: []Filter{
				{
					Kind:   &msgbus.ObjectStatusUpdated{},
					Labels: []pubsub.Label{{"path", "root/svc/foo"}},
				},
			},
		},
		",": {
			filterS:  []string{","},
			expected: nil,
		},
		"types and labels": {
			filterS: []string{"ObjectStatusUpdated,path=root/svc/foo", "ConfigFileRemoved,path=root/svc/bar"},
			expected: []Filter{
				{
					Kind:   &msgbus.ObjectStatusUpdated{},
					Labels: []pubsub.Label{{"path", "root/svc/foo"}},
				},
				{
					Kind:   &msgbus.ConfigFileRemoved{},
					Labels: []pubsub.Label{{"path", "root/svc/bar"}},
				},
			},
		},
		"type label and matcher": {
			filterS: []string{"InstanceStatusUpdated,node=nodeX,path=root/svc/foo,.data.instance_status.overall=down"},
			expected: []Filter{
				{
					Kind:        &msgbus.InstanceStatusUpdated{},
					Labels:      []pubsub.Label{{"node", "nodeX"}, {"path", "root/svc/foo"}},
					DataFilters: DataFilters{{Key: ".data.instance_status.overall", Op: "=", Value: "down"}},
				},
			},
		},
		"type label and matchers": {
			filterS: []string{
				"InstanceStatusUpdated,node=nodeX,path=root/svc/foo" +
					",.data.instance_status.overall=down" +
					",.data.instance_status.avail=stdby up",
			},
			expected: []Filter{
				{
					Kind:   &msgbus.InstanceStatusUpdated{},
					Labels: []pubsub.Label{{"node", "nodeX"}, {"path", "root/svc/foo"}},
					DataFilters: DataFilters{
						{Key: ".data.instance_status.overall", Op: "=", Value: "down"},
						{Key: ".data.instance_status.avail", Op: "=", Value: "stdby up"},
					},
				},
			},
		},
		"type but no label": {
			filterS: []string{"ObjectStatusUpdated"},
			expected: []Filter{
				{
					Kind: &msgbus.ObjectStatusUpdated{},
				},
			},
		},
		"only label": {
			filterS: []string{"path=root/svc/foo"},
			expected: []Filter{
				{
					Labels: []pubsub.Label{{"path", "root/svc/foo"}},
				},
			},
		},
		"only label with heading comma": {
			filterS: []string{",path=root/svc/foo"},
			expected: []Filter{
				{
					Labels: []pubsub.Label{{"path", "root/svc/foo"}},
				},
			},
		},
		"only labels": {
			filterS: []string{",path=root/svc/foo", ",path=root/svc/bar"},
			expected: []Filter{
				{
					Labels: []pubsub.Label{{"path", "root/svc/foo"}},
				},
				{
					Labels: []pubsub.Label{{"path", "root/svc/bar"}},
				},
			},
		},
		"mix type and label": {
			filterS: []string{"ObjectStatusUpdated", ",path=root/svc/bar"},
			expected: []Filter{
				{
					Kind: &msgbus.ObjectStatusUpdated{},
				},
				{
					Labels: []pubsub.Label{{"path", "root/svc/bar"}},
				},
			},
		},
		"all filter": {
			filterS:  []string{},
			expected: []Filter(nil),
		},
		" null filter": {
			filterS:  []string{""},
			expected: []Filter(nil),
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := api.GetDaemonEventsParams{
				Filter: &c.filterS,
			}
			filters, err := parseFilters(p)
			require.Nil(t, err)
			require.Equal(t, c.expected, filters)
			require.Len(t, filters, len(c.expected))
		})
	}
}

func TestGetDaemonEventsBadParams(t *testing.T) {
	cases := map[string]struct {
		filterS []string
		err     error
	}{
		"invalid kind": {
			filterS: []string{"Plop"},
			err:     fmt.Errorf("can't find type for kind: Plop"),
		},
		"missing label key": {
			filterS: []string{",=foo"},
			err:     fmt.Errorf("invalid label filter expression: =foo (empty key)"),
		},
		"multiple value filters for same kind": {
			filterS: []string{"InstanceStatusUpdated,.data.instance_status=up", "InstanceStatusUpdated"},
			err:     fmt.Errorf("can't filter same kind multiple times when it has a value matcher: InstanceStatusUpdated"),
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := api.GetDaemonEventsParams{
				Filter: &c.filterS,
			}
			_, err := parseFilters(p)
			require.Equal(t, c.err, err)
		})
	}
}

func TestDataFilters(t *testing.T) {
	msg := msgbus.InstanceStatusUpdated{Value: instance.Status{Overall: status.Down, Avail: status.StandbyDown}}
	ev := event.ToEvent(&msg, 1)
	v := make(map[string]any)
	if err := json.Unmarshal(ev.Data, &v); err != nil {
		return
	}

	matched := map[string]DataFilters{
		"match uniq value": {{Key: ".instance_status.overall", Op: "=", Value: string("\"down\"")}},
		"matched both values": DataFilters{
			{Key: ".instance_status.overall", Op: "=", Value: string("\"down\"")},
			{Key: ".instance_status.avail", Op: "=", Value: string("\"stdby down\"")},
		},
	}

	notMatched := map[string]DataFilters{
		"unmatched the value": {{Key: ".instance_status.overall", Op: "=", Value: string("\"up\"")}},
		"unmatched second value": DataFilters{
			{Key: ".instance_status.overall", Op: "=", Value: string("\"down\"")},
			{Key: ".instance_status.avail", Op: "=", Value: string("\"stdby up\"")},
		},
		"unmatched first value": DataFilters{
			{Key: ".instance_status.overall", Op: "=", Value: string("\"up\"")},
			{Key: ".instance_status.avail", Op: "=", Value: string("\"stdby down\"")},
		},
		"unmatched on both values": DataFilters{
			{Key: ".instance_status.overall", Op: "=", Value: string("\"up\"")},
			{Key: ".instance_status.avail", Op: "=", Value: string("\"up\"")},
		},
	}

	for s, c := range matched {
		t.Run(s, func(t *testing.T) {
			require.True(t, c.match(v))
		})
	}

	for s, c := range notMatched {
		t.Run(s, func(t *testing.T) {
			require.False(t, c.match(v))
		})
	}
}
