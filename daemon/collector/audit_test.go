package collector

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/util/auditstate"
	"github.com/opensvc/om3/v3/util/plog"
)

// An audit session of the collector begun before the collector subscribes,
// as one reconnecting to a daemon restarting, is attached from the registry:
// its AuditStart went to no collector.
func TestAttachesAnAuditBegunBeforeTheCollector(t *testing.T) {
	tr, _ := newTestT(t)
	reg := &auditstate.Registry{}
	tr.ctx = daemonctx.WithAuditRegistry(context.Background(), reg)
	q := make(chan plog.LogMessage, 10)
	_, err := reg.Begin(context.Background(), q, []string{"collector"}, make(chan struct{}), "root", false, nil)
	require.NoError(t, err)

	tr.attachActiveAuditIfAny()
	tr.log.Tracef("audited after the attach")

	deadline := time.After(time.Second)
	for {
		select {
		case msg := <-q:
			if strings.Contains(msg.Message, "audited after the attach") {
				return
			}
		case <-deadline:
			t.Fatal("the collector logs do not reach the audit session")
		}
	}
}
