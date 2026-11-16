//go:build linux

package resip

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/network"
)

func TestPublishInfo(t *testing.T) {
	drawn, err := network.ParsePublishes([]string{"80", "25489:81/udp"})
	require.NoError(t, err)
	drawn[0].HostPort, drawn[0].Auto = 30123, true

	cases := []struct {
		name      string
		resolved  []network.Publish
		publish   []string
		expose    []string
		published []string
		exposed   []string
	}{
		{
			name:      "started, every published port exposed",
			resolved:  drawn,
			publish:   []string{"80", "25489:81/udp"},
			published: []string{"30123:80/tcp", "25489:81/udp"},
			exposed:   []string{"30123:80/tcp", "25489:81/udp"},
		},
		{
			name:      "not started, a port to draw exposes nothing yet",
			publish:   []string{"80", "25489:81/udp"},
			published: []string{"80", "25489:81/udp"},
			exposed:   []string{"25489:81/udp"},
		},
		{
			name:      "an expose replaces the published ports",
			resolved:  drawn,
			publish:   []string{"80", "25489:81/udp"},
			expose:    []string{"5432/tcp"},
			published: []string{"30123:80/tcp", "25489:81/udp"},
			exposed:   []string{"5432/tcp"},
		},
		{
			name:      "none exposes nothing",
			resolved:  drawn,
			publish:   []string{"80", "25489:81/udp"},
			expose:    []string{"none"},
			published: []string{"30123:80/tcp", "25489:81/udp"},
			exposed:   []string{},
		},
		{
			name:    "no publish, the expose as it was",
			expose:  []string{"53/udp"},
			exposed: []string{"53/udp"},
		},
		{
			name:    "no publish nor expose",
			exposed: []string{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			published, exposed := publishInfo(c.resolved, c.publish, c.expose)
			assert.Equal(t, c.published, published)
			assert.Equal(t, c.exposed, exposed)
		})
	}
}

func TestExposeInfo(t *testing.T) {
	assert.Equal(t, []string{}, ExposeInfo([]string{"none"}))
	assert.Equal(t, []string{"53/udp"}, ExposeInfo([]string{"53/udp"}))
	assert.Nil(t, ExposeInfo(nil))
}
