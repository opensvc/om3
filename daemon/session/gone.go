package session

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
)

// goneTimeout bounds the wait of a delete or a purge for the instances of
// the object to be gone once its participants left: past it, the
// orchestration ends failed, naming the nodes holding one still.
var goneTimeout = 10 * time.Second

// awaitingGone is the deleting orchestrations whose participants all left,
// by id, waiting since then for the object to be gone. The caller holds the
// lock.
var awaitingGone = make(map[string]time.Time)

// remainingInstances returns the nodes, but the ones not waited for, still
// holding an instance of the object of path: its config, status or monitor.
func remainingInstances(path string) []string {
	p, err := naming.ParsePath(path)
	if err != nil {
		return nil
	}
	m := make(map[string]bool)
	for nodename := range instance.ConfigData.GetByPath(p) {
		m[nodename] = true
	}
	for nodename := range instance.StatusData.GetByPath(p) {
		m[nodename] = true
	}
	for nodename := range instance.MonitorData.GetByPath(p) {
		m[nodename] = true
	}
	var l []string
	for nodename := range m {
		// A node without a monitor, forgotten or not heard from since
		// this daemon started, runs no orchestration either, nor does a
		// node upgrading its agent.
		if mon := node.MonitorData.GetByNode(nodename); mon == nil || mon.State.IsAwayFromObjects() {
			continue
		}
		l = append(l, nodename)
	}
	slices.Sort(l)
	return l
}

// endIfGone ends the deleting orchestration of id, its participants all
// left, once no instance of its object remains, failed past goneTimeout. It
// returns false while it waits. The caller holds the lock.
//
// The participants are the nodes seen running the orchestration: a node
// whose instance came back just before, as one fetching the config again,
// was not one yet as the others left, and the delete or the purge was said
// over with an instance left (qatests 308).
func endIfGone(id string, o *Orchestration, now time.Time) bool {
	since, waiting := awaitingGone[id]
	remaining := remainingInstances(o.Path)
	switch {
	case len(remaining) == 0:
		delete(awaitingGone, id)
		return true
	case !waiting:
		awaitingGone[id] = now
		return false
	case now.Sub(since) >= goneTimeout:
		delete(awaitingGone, id)
		failures[id] = append(failures[id], fmt.Sprintf("instances remain on %s after %s", strings.Join(remaining, ", "), goneTimeout))
		return true
	default:
		return false
	}
}

// CheckGone ends the deleting orchestrations waiting for their object to be
// gone, once it is, or past the timeout.
func CheckGone() {
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	for id := range awaitingGone {
		o, ok := orchestrations[id]
		if !ok || o.EndedAt != nil {
			delete(awaitingGone, id)
			continue
		}
		if endIfGone(id, o, now) {
			finish(id, o, now)
		}
	}
}

// awaitingAny says whether a deleting orchestration waits for its object to
// be gone.
func awaitingAny() bool {
	mu.Lock()
	defer mu.Unlock()
	return len(awaitingGone) > 0
}
