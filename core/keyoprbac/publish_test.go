package keyoprbac

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// TestPublishOpenForADrawnPortOnly pins that a user with no grant publishes
// ports of the node the kernel draws, and names none: neither a port nor an
// address of the node.
func TestPublishOpenForADrawnPortOnly(t *testing.T) {
	for _, value := range []string{
		"80",
		"80/tcp",
		"53/udp",
		"80/tcp 53/udp 8080",
		"",
	} {
		assert.NoErrorf(t, Denied(noGrant, naming.KindSvc, "ip#1", "publish", value, section("network", "publish")),
			"publish=%q names no port nor address of the node", value)
	}
	for _, value := range []string{
		"25489:80",
		"25489:80/tcp",
		"80/tcp 22:22",
		"10.0.0.5::80",
		"10.0.0.5:8443:443",
		"[fd00::1]::80",
		"{env.port}:80",
		"{env.ports}",
		"80/sctp",
		"x",
	} {
		assert.Errorf(t, Denied(noGrant, naming.KindSvc, "ip#1", "publish", value, section("network", "publish")),
			"publish=%q names a port or an address of the node, or may", value)
		assert.NoErrorf(t, Denied(rbac.Grants{rbac.GrantRoot}, naming.KindSvc, "ip#1", "publish", value, section("network", "publish")),
			"root publishes %q", value)
	}
}
