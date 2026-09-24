package daemonapi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// A configuration file the daemon wrote is announced at once, labeled the way
// the instance config manager of the object listens for it: announced with
// other labels, it would wait for the filesystem watcher, 200ms later, as
// before.
func TestAConfigFileWrittenIsAnnouncedToItsInstanceConfigManager(t *testing.T) {
	bus := pubsub.NewBus(t.Name())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus.Start(ctx)
	defer bus.Stop()

	p := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "written"}
	sub := bus.Sub(t.Name())
	sub.AddFilter(&msgbus.ConfigFileUpdated{}, pubsub.Label{"path", p.String()})
	sub.Start()
	defer func() { _ = sub.Stop() }()

	a := &DaemonAPI{Bus: bus}
	a.announceConfigFileWritten(p)

	select {
	case i := <-sub.C:
		c, ok := i.(*msgbus.ConfigFileUpdated)
		require.True(t, ok)
		require.Equal(t, p, c.Path)
		require.Equal(t, p.ConfigFile(), c.File)
	case <-time.After(time.Second):
		t.Fatal("the write was not announced to the listener of its path")
	}
}
