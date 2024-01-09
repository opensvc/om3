package daemonapi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

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
		"type but no label": {
			filterS: []string{"ObjectStatusUpdated"},
			expected: []Filter{
				{
					Kind: &msgbus.ObjectStatusUpdated{},
				},
			},
		},
		"only label": {
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
		"missing kind": {
			filterS: []string{"path=foo"},
			err:     fmt.Errorf("can't find type for kind: path=foo"),
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
