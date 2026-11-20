package daemonapi

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/netstatus"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/pubsub"
)

func TestGetDaemonEventsParamsOk(t *testing.T) {
	cases := map[string]struct {
		filterS  []string
		expected []Filter
	}{
		"type and label": {
			filterS: []string{"ObjectStatusUpdated,path=ns1/svc/foo"},
			expected: []Filter{
				{
					Kind:   &msgbus.ObjectStatusUpdated{},
					Labels: []pubsub.Label{{"path", "ns1/svc/foo"}},
				},
			},
		},
		",": {
			filterS:  []string{","},
			expected: nil,
		},
		"types and labels": {
			filterS: []string{"ObjectStatusUpdated,path=ns1/svc/foo", "ConfigFileRemoved,path=ns2/svc/bar"},
			expected: []Filter{
				{
					Kind:   &msgbus.ObjectStatusUpdated{},
					Labels: []pubsub.Label{{"path", "ns1/svc/foo"}},
				},
				{
					Kind:   &msgbus.ConfigFileRemoved{},
					Labels: []pubsub.Label{{"path", "ns2/svc/bar"}},
				},
			},
		},
		"type label and matcher": {
			filterS: []string{"InstanceStatusUpdated,node=nodeX,path=ns1/svc/foo,.data.instance_status.overall=down"},
			expected: []Filter{
				{
					Kind:        &msgbus.InstanceStatusUpdated{},
					Labels:      []pubsub.Label{{"node", "nodeX"}, {"path", "ns1/svc/foo"}},
					DataFilters: DataFilters{{Key: "data.instance_status.overall", Op: "=", Value: "down"}},
				},
			},
		},
		"type label and matchers": {
			filterS: []string{
				"InstanceStatusUpdated,node=nodeX,path=ns1/svc/foo" +
					",.data.instance_status.overall=down" +
					",.data.instance_status.avail=stdby up",
			},
			expected: []Filter{
				{
					Kind:   &msgbus.InstanceStatusUpdated{},
					Labels: []pubsub.Label{{"node", "nodeX"}, {"path", "ns1/svc/foo"}},
					DataFilters: DataFilters{
						{Key: "data.instance_status.overall", Op: "=", Value: "down"},
						{Key: "data.instance_status.avail", Op: "=", Value: "stdby up"},
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
			filterS: []string{"path=ns1/svc/foo"},
			expected: []Filter{
				{
					Labels: []pubsub.Label{{"path", "ns1/svc/foo"}},
				},
			},
		},
		"only label with heading comma": {
			filterS: []string{",path=ns1/svc/foo"},
			expected: []Filter{
				{
					Labels: []pubsub.Label{{"path", "ns1/svc/foo"}},
				},
			},
		},
		"only labels": {
			filterS: []string{",path=ns1/svc/foo", ",path=ns1/svc/bar"},
			expected: []Filter{
				{
					Labels: []pubsub.Label{{"path", "ns1/svc/foo"}},
				},
				{
					Labels: []pubsub.Label{{"path", "ns1/svc/bar"}},
				},
			},
		},
		"mix type and label": {
			filterS: []string{"ObjectStatusUpdated", ",path=ns1/svc/bar"},
			expected: []Filter{
				{
					Kind: &msgbus.ObjectStatusUpdated{},
				},
				{
					Labels: []pubsub.Label{{"path", "ns1/svc/bar"}},
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
		"match uniq value": {{Key: "instance_status.overall", Op: "=", Value: string("\"down\"")}},
		"matched both values": DataFilters{
			{Key: "instance_status.overall", Op: "=", Value: string("\"down\"")},
			{Key: "instance_status.avail", Op: "=", Value: string("\"stdby down\"")},
		},
	}

	notMatched := map[string]DataFilters{
		"unmatched the value": {{Key: "instance_status.overall", Op: "=", Value: string("\"up\"")}},
		"unmatched second value": DataFilters{
			{Key: "instance_status.overall", Op: "=", Value: string("\"down\"")},
			{Key: "instance_status.avail", Op: "=", Value: string("\"stdby up\"")},
		},
		"unmatched first value": DataFilters{
			{Key: "instance_status.overall", Op: "=", Value: string("\"up\"")},
			{Key: "instance_status.avail", Op: "=", Value: string("\"stdby down\"")},
		},
		"unmatched on both values": DataFilters{
			{Key: "instance_status.overall", Op: "=", Value: string("\"up\"")},
			{Key: "instance_status.avail", Op: "=", Value: string("\"up\"")},
		},
	}

	for s, c := range matched {
		t.Run(s, func(t *testing.T) {
			require.Truef(t, c.match(v), "'%s' should match '%s'", v, c)
		})
	}

	for s, c := range notMatched {
		t.Run(s, func(t *testing.T) {
			require.Falsef(t, c.match(v), "'%s' shouldn't match '%s'", v, c)
		})
	}
}

func TestIsEventAllowedForGuest(t *testing.T) {
	grants := rbac.NewGrants("guest:ns1")
	cases := map[string]struct {
		labels  pubsub.Labels
		allowed bool
	}{
		"namespace label of a granted namespace": {pubsub.NewLabels("node", "n1", "namespace", "ns1", "path", "ns1/svc/s1"), true},
		"namespace label of another namespace":   {pubsub.NewLabels("node", "n1", "namespace", "ns2", "path", "ns2/svc/s1"), false},
		"cached event of a granted namespace":    {pubsub.NewLabels("node", "n1", "path", "ns1/svc/s1", "source", "cache"), true},
		"cached event of another namespace":      {pubsub.NewLabels("node", "n1", "path", "ns2/svc/s1", "source", "cache"), false},
		"cached event of the root namespace":     {pubsub.NewLabels("node", "n1", "path", "s1", "source", "cache"), false},
		"cached event of an unparsable path":     {pubsub.NewLabels("node", "n1", "path", "ns2/badkind/s1", "source", "cache"), false},
		"event of the node configuration":        {pubsub.NewLabels("node", "n1", "path", ""), true},
		"event with neither namespace nor path":  {pubsub.NewLabels("node", "n1"), true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			msg := &msgbus.InstanceStatusUpdated{Msg: pubsub.Msg{Labels: c.labels}}
			require.Equal(t, c.allowed, isEventAllowedForGuest(grants, msg))
		})
	}
}

// A guest of the namespaces a pattern matches gets their events, the ones of
// the namespaces created after the grant was written among them.
func TestIsEventAllowedForAGuestOfAPattern(t *testing.T) {
	grants := rbac.NewGrants("guest:prod*")
	for ns, allowed := range map[string]bool{"prod": true, "prod-eu": true, "dev": false, "system": false} {
		msg := &msgbus.InstanceStatusUpdated{Msg: pubsub.Msg{Labels: pubsub.NewLabels("namespace", ns)}}
		require.Equal(t, allowed, isEventAllowedForGuest(grants, msg), ns)
	}
}

// The network setup status of the nodes, the addresses the nodes reach each
// other at, is read by the root and inspector grants only: its events carry
// no namespace, and reached every guest.
func TestNetworkSetupIsReadByRootAndInspectorOnly(t *testing.T) {
	for grants, sees := range map[string]bool{
		"root":                 true,
		"inspector":            true,
		"guest,inspector":      true,
		"guest":                false,
		"guest:ns1":            false,
		"admin":                false,
		"scraper":              false,
		"inspector:ns1,guest":  false,
		"operator,prioritizer": false,
	} {
		assert.Equal(t, sees, seesNetworkSetup(rbac.NewGrants(strings.Split(grants, ",")...)), grants)
	}
	for _, msg := range []pubsub.Messager{&msgbus.NodeNetworkStatusUpdated{}, &msgbus.NodeNetworkStatusDeleted{}} {
		assert.True(t, isNetworkSetupEvent(msg), "%T", msg)
	}
	for _, msg := range []pubsub.Messager{&msgbus.NetworkPeersUpdated{}, &msgbus.NodePoolStatusUpdated{}, &msgbus.InstanceStatusUpdated{}} {
		assert.False(t, isNetworkSetupEvent(msg), "%T", msg)
	}
}

// The node data of a NodeDataUpdated carries no namespace, and gave every
// guest the instances of every namespace and how the networks are set up on
// the node. A subscriber reads a copy without the network setup unless it
// holds the root or inspector grant, and with the instances of its
// namespaces only unless it holds a role on every namespace. The message,
// shared with the other subscribers, is left untouched.
func TestNodeDataIsRedactedForTheSubscriber(t *testing.T) {
	newMsg := func() *msgbus.NodeDataUpdated {
		return &msgbus.NodeDataUpdated{Node: "n1", Value: node.Node{
			Instance: map[string]instance.Instance{"ns1/svc/a": {}, "ns2/svc/b": {}, "svc/c": {}},
			Network:  map[string]netstatus.Status{"l2": {}},
		}}
	}
	view := func(grants string) *msgbus.NodeDataUpdated {
		userGrants := rbac.NewGrants(strings.Split(grants, " ")...)
		seesAll := userGrants.HasRoleOn("", rbac.RoleRoot, rbac.RoleAdmin, rbac.RoleOperator, rbac.RoleGuest, rbac.RoleJoin, rbac.RoleLeave)
		return redactNodeData(newMsg(), seesNetworkSetup(userGrants), seesAll, guestOn(userGrants)).(*msgbus.NodeDataUpdated)
	}
	keys := func(m map[string]instance.Instance) []string {
		l := make([]string, 0, len(m))
		for k := range m {
			l = append(l, k)
		}
		sort.Strings(l)
		return l
	}
	all := []string{"ns1/svc/a", "ns2/svc/b", "svc/c"}

	v := view("guest:ns1")
	assert.Equal(t, []string{"ns1/svc/a"}, keys(v.Value.Instance))
	assert.Nil(t, v.Value.Network)

	v = view("guest")
	assert.Equal(t, all, keys(v.Value.Instance))
	assert.Nil(t, v.Value.Network)

	v = view("guest:ns1 inspector")
	assert.Equal(t, []string{"ns1/svc/a"}, keys(v.Value.Instance))
	assert.Len(t, v.Value.Network, 1)

	v = view("root")
	assert.Equal(t, all, keys(v.Value.Instance))
	assert.Len(t, v.Value.Network, 1)

	msg := newMsg()
	_ = redactNodeData(msg, false, false, guestOn(rbac.NewGrants("guest:ns1")))
	assert.Len(t, msg.Value.Instance, 3, "the shared message is untouched")
	assert.Len(t, msg.Value.Network, 1, "the shared message is untouched")

	other := &msgbus.NodeStatusUpdated{}
	assert.Same(t, other, redactNodeData(other, false, false, guestOn(nil)), "other events are left as is")
}
