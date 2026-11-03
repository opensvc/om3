package resiphost

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/util/ping"
)

// An address answering is a conflict of a provision starting it, on the
// leader, but not of a provision off the leader, which does not start it: the
// address answering is then the one the leader of the object started.
func TestTheProvisionOffTheLeaderDoesNotPingItsAddress(t *testing.T) {
	if alive, err := ping.Ping("127.0.0.1", 2*time.Second); err != nil || !alive {
		t.Skip("no ping of the loopback here")
	}
	newResource := func() *T {
		r := &T{Dev: "osvc-test-nodev"}
		r._ipaddr = net.ParseIP("127.0.0.1")
		return r
	}
	leader := actioncontext.WithLeader(context.Background(), true)
	assert.NotEmpty(t, newResource().ProvisionConflicts(leader), "on the leader, the address answering is a conflict")
	nonLeader := actioncontext.WithLeader(context.Background(), false)
	assert.Empty(t, newResource().ProvisionConflicts(nonLeader), "off the leader, the address is the leader's")
}
