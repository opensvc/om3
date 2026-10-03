package collector

import (
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// The pending instance actions of the local node.
//
// The action process writes the begin and the end of an action to the
// pending directory, and signals the local daemon (PostInstanceCollectorAction),
// which publishes an InstanceActionPending the collector speaker sends.
//
// The collector goroutine of every node owns the pending directory of its
// node: it announces again the actions the speaker did not acknowledge, and
// drops their files once the speaker acknowledges them with an
// InstanceActionSent. So no action is lost to a daemon down during the
// action, to the lack of a speaker, to a speaker change or restart, nor to a
// collector down.

var (
	// actionPendingExpire is the age above which the files of an action not
	// acknowledged are removed, so a collector gone for good does not fill
	// the pending directory.
	actionPendingExpire = 24 * time.Hour

	// actionAnnounceInterval is the minimum interval between two announces
	// of an action not acknowledged.
	actionAnnounceInterval = time.Minute
)

// ActionPendingDir returns the pending directory of the local instance
// actions.
func ActionPendingDir() collector.ActionPendingDir {
	return collector.ActionPendingDir(rawconfig.CollectorActionPendingDir())
}

// NewInstanceActionPending returns the InstanceActionPending announcing the
// phase of the action of key, read from the pending directory dir of the
// node nodename.
//
// The announce of an end carries the collector uuid of its begin, when the
// begin was acknowledged.
func NewInstanceActionPending(dir collector.ActionPendingDir, key string, phase collector.ActionPhase, nodename string) (*msgbus.InstanceActionPending, error) {
	a, err := dir.Read(key, phase)
	if err != nil {
		return nil, err
	}
	if a.Phase() != phase {
		return nil, fmt.Errorf("pending %s file of %s holds a %s", phase, key, a.Phase())
	}
	msg := &msgbus.InstanceActionPending{
		Path:   a.Path,
		Node:   nodename,
		Phase:  phase,
		Action: a,
	}
	if phase == collector.ActionPhaseEnd {
		if msg.UUID, err = dir.ReadUUID(key); err != nil {
			return nil, err
		}
	}
	return msg, nil
}

// InstanceActionPendingLabels returns the labels to publish msg with. The
// node label makes the daemon data forward it to the peers.
func InstanceActionPendingLabels(msg *msgbus.InstanceActionPending) []pubsub.Label {
	return []pubsub.Label{
		{"node", msg.Node},
		{"namespace", msg.Path.Namespace},
		{"path", msg.Path.String()},
	}
}

// announceActionPending publishes an InstanceActionPending for each action
// of the pending directory, and removes the files of the actions expired.
//
// A node without collector configuration only removes the expired actions.
//
// Unless force is set, an action announced, or written, less than
// actionAnnounceInterval ago is skipped: it waits for the speaker.
//
// The end of an action is announced in place of its begin: the collector
// builds the full record from it. An action whose begin was acknowledged,
// and still running, has nothing to announce.
func (t *T) announceActionPending(force bool) {
	keys, err := t.actionPendingDir.List()
	if err != nil {
		t.log.Warnf("list pending actions: %s", err)
		return
	}
	now := time.Now()
	for _, k := range keys {
		if now.Sub(k.ModTime) > actionPendingExpire {
			t.log.Warnf("drop the action %s not acknowledged by the collector since %s", k.Key, k.ModTime)
			if err := t.actionPendingDir.RemoveAll(k.Key); err != nil {
				t.log.Warnf("drop the action %s: %s", k.Key, err)
			}
			delete(t.actionAnnouncedAt, k.Key)
			continue
		}
		if t.disable {
			// no collector to report to: only expire
			continue
		}
		var phase collector.ActionPhase
		switch {
		case k.HasEnd:
			phase = collector.ActionPhaseEnd
		case k.HasBegin:
			phase = collector.ActionPhaseBegin
		default:
			// the begin was acknowledged and the action still runs: keep
			// the collector uuid for its end, nothing to announce.
			continue
		}
		if !force {
			last := k.ModTime
			if at, ok := t.actionAnnouncedAt[k.Key]; ok && at.After(last) {
				last = at
			}
			if now.Sub(last) < actionAnnounceInterval {
				continue
			}
		}
		msg, err := NewInstanceActionPending(t.actionPendingDir, k.Key, phase, t.localhost)
		if err != nil {
			t.log.Warnf("announce the %s of the action %s: %s", phase, k.Key, err)
			continue
		}
		t.log.Tracef("announce the %s of the action %s", phase, k.Key)
		t.publisher.Pub(msg, InstanceActionPendingLabels(msg)...)
		t.actionAnnouncedAt[k.Key] = now
	}
}

// onLocalInstanceActionSent drops the pending files the collector speaker
// acknowledged.
//
// The begin acknowledged leaves the collector uuid, for the end to send it
// back. The end acknowledged drops all the files of the action.
func (t *T) onLocalInstanceActionSent(c *msgbus.InstanceActionSent) {
	k := collector.ActionKey(c.ExecID, c.Path)
	switch c.Phase {
	case collector.ActionPhaseBegin:
		if c.UUID != "" {
			if err := t.actionPendingDir.WriteUUID(k, c.UUID); err != nil {
				t.log.Warnf("save the collector uuid of the action %s: %s", k, err)
			}
		}
		if err := t.actionPendingDir.RemoveBegin(k); err != nil {
			t.log.Warnf("drop the begin of the action %s: %s", k, err)
		}
	case collector.ActionPhaseEnd:
		if err := t.actionPendingDir.RemoveAll(k); err != nil {
			t.log.Warnf("drop the action %s: %s", k, err)
		}
		delete(t.actionAnnouncedAt, k)
	}
}
