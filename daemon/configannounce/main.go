// Package configannounce announces the configuration files the daemon writes
// itself, and lets the filesystem watcher tell them from the writes it has to
// announce.
//
// The watcher debounces the events of a file for 200ms, which it has to for
// the writes it cannot tell apart, an editor saving in several steps. A write
// of the daemon is one, done when it is announced, so the daemon does not
// wait for the watcher to say it.
//
// The watcher still sees the write, 200ms later. Announced again, it is not
// the no-op it looks like: the configuration manager of the object may have
// ended in between, on a configuration whose scope left this node, and the
// second announce starts another one, which publishes the configuration for
// its peers a second time. A peer that had installed it on the first one
// takes the second as a sign its own file is going away, and does not
// recover it when it is removed later. So the watcher consumes the record of
// the announce instead of announcing the write again.
package configannounce

import (
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/pubsub"
)

var (
	mu sync.Mutex

	// announced is the modification time each configuration file had when
	// the daemon announced it, until the watcher consumes it.
	announced = make(map[string]time.Time)
)

// Written publishes ConfigFileUpdated for the configuration file of p, which
// the daemon has just written, and records it announced at its current
// modification time.
func Written(publisher pubsub.Publisher, p naming.Path) {
	filename := p.ConfigFile()
	if mtime := file.ModTime(filename); !mtime.IsZero() {
		mu.Lock()
		announced[filename] = mtime
		mu.Unlock()
	}
	publisher.Pub(&msgbus.ConfigFileUpdated{Path: p, File: filename},
		pubsub.Label{"namespace", p.Namespace},
		pubsub.Label{"path", p.String()},
	)
}

// Consume tells whether the configuration file at filename was announced by
// Written at the modification time mtime, and drops the record: one
// announce stands for one write, so it hides one event of the watcher.
func Consume(filename string, mtime time.Time) bool {
	mu.Lock()
	defer mu.Unlock()
	at, ok := announced[filename]
	if !ok {
		return false
	}
	delete(announced, filename)
	return !mtime.IsZero() && at.Equal(mtime)
}

// Forget drops the record of the configuration file at filename, removed:
// a file put back later at the same modification time, from a backup, is a
// write of its own.
func Forget(filename string) {
	mu.Lock()
	defer mu.Unlock()
	delete(announced, filename)
}
