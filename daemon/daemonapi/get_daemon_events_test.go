package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func TestGetDaemonEventsParams(t *testing.T) {
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
		"only labels": {
			filterS: []string{"path=root/svc/foo", "path=root/svc/bar"},
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
			filterS: []string{"ObjectStatusUpdated", "path=root/svc/bar"},
			expected: []Filter{
				{
					Kind: &msgbus.ObjectStatusUpdated{},
				},
				{
					Labels: []pubsub.Label{{"path", "root/svc/bar"}},
				},
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := GetDaemonEventsParams{
				Filter: &c.filterS,
			}
			filters, err := p.parseFilters()
			require.Nil(t, err)
			require.Equal(t, c.expected, filters)
			require.Len(t, filters, len(c.expected))
		})
	}
}
