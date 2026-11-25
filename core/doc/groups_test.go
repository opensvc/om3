package doc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
)

// A strategy is served with the keywords standing for the ones it requires,
// for a client not to ask the dev of a static address naming its network,
// and with an empty list rather than null when it has none.
func TestConvertKeywordGroupsServesRequiresUnless(t *testing.T) {
	items := ConvertKeywordGroups([]keywords.DriverGroup{
		{
			Driver: "ip.netns",
			Group: keywords.Group{
				ID: "addressing",
				Strategies: []keywords.Strategy{
					{ID: "static", Uses: []string{"name"}, Requires: []string{"dev"}, RequiresUnless: []string{"network"}},
					{ID: "ipam", Uses: []string{"network"}},
				},
			},
		},
	})
	require.Len(t, items, 1)
	require.Len(t, items[0].Strategies, 2)
	assert.Equal(t, []string{"dev"}, items[0].Strategies[0].Requires)
	assert.Equal(t, []string{"network"}, items[0].Strategies[0].RequiresUnless)
	assert.NotNil(t, items[0].Strategies[1].RequiresUnless)
	assert.Empty(t, items[0].Strategies[1].RequiresUnless)
}
