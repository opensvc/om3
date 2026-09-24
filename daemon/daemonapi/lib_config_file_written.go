package daemonapi

import (
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// announceConfigFileWritten says that this daemon wrote the configuration
// file of p, without waiting for the filesystem watcher to notice.
//
// The watcher debounces the events of a file for 200ms, which it has to for
// the writes it cannot tell apart, an editor saving in several steps. A write
// of the daemon is one, and done when this is called, and the 200ms were
// most of the time a configuration took to reach the other nodes: the peers
// hear of a configuration when this node publishes it, which it only did
// once the watcher had spoken.
//
// The watcher still speaks, 200ms later, and finds the file already read at
// that modification time, which does nothing.
func (a *DaemonAPI) announceConfigFileWritten(p naming.Path) {
	a.Bus.Pub(&msgbus.ConfigFileUpdated{Path: p, File: p.ConfigFile()},
		pubsub.Label{"namespace", p.Namespace},
		pubsub.Label{"path", p.String()},
	)
}
