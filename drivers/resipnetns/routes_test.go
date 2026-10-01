//go:build linux

package resipnetns

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The default route of the namespace is set only when it has none, as v2
// does: the namespace is often shared with an ip.cni resource of the same
// container, which installs the default route of its network, and replacing
// it broke the traffic the container routes through that network.
func TestDefaultRouteToSet(t *testing.T) {
	const cni = "default via 10.22.0.1 dev eth0 \n"
	const onLink = "default dev eth3 scope link \n"

	for _, tc := range []struct {
		name     string
		defaults string
		gateway  string
		want     []string
	}{
		{
			name:     "the default route of an ip.cni of the namespace is kept",
			defaults: cni,
			want:     nil,
		},
		{
			name:     "a default route through a gateway is kept, whatever the gateway keyword",
			defaults: cni,
			gateway:  "45.13.156.1",
			want:     nil,
		},
		{
			name:     "a default route through an interface is kept",
			defaults: onLink,
			want:     nil,
		},
		{
			name:     "the gateway keyword replaces a default route through an interface",
			defaults: onLink,
			gateway:  "45.13.156.1",
			want:     []string{"default", "via", "45.13.156.1"},
		},
		{
			name: "no default route: through the interface",
			want: []string{"default", "dev", "eth7"},
		},
		{
			name:    "no default route: through the gateway",
			gateway: "45.13.156.1",
			want:    []string{"default", "via", "45.13.156.1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, defaultRouteToSet(tc.defaults, tc.gateway, "eth7"))
		})
	}
}
