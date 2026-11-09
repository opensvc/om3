package collector

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
)

// speakerGracePeriod returns how long the nodes wait for a speaker gone, as
// one whose daemon restarts, before another takes the speech: a speaker back
// within it speaks again, with no speaker change, no overlap of two speakers
// and no full dataset sent twice. The actions and the changes of the nodes
// wait for it, the actions on the disk of their node.
//
// It is the collector.speaker_grace_period of the configuration applied, the
// default when not set.
func (t *T) speakerGracePeriod() time.Duration {
	if t.appliedConfig == nil {
		return collector.DefaultSpeakerGracePeriod
	}
	return collector.BoundSpeakerGracePeriod(t.appliedConfig.SpeakerGracePeriod)
}

// isAway says whether the collector of the peer nodename is away, rather than
// unable to speak: its node gone, its collector stopping with its daemon, or
// starting again, or its state not known yet. A collector disabled, without
// client, or whose credentials the collector refuses, is unable to speak, not
// away, and so is a collector stopped, which did not feed the collector.
func isAway(nodename string) bool {
	if node.MonitorData.GetByNode(nodename) == nil {
		return true
	}
	c := daemonsubsystem.DataCollector.Get(nodename)
	if c == nil {
		return true
	}
	switch c.State {
	case "", "stopping", "starting":
		return true
	default:
		return false
	}
}

// awaySpeaker returns the speaker the node waits for, away for less than the
// speaker grace period, "" when it waits for none: the speaker is the node
// itself, or is not away, or is away for longer.
func (t *T) awaySpeaker(now time.Time) string {
	if t.speaker == "" || t.speaker == t.localhost || !isAway(t.speaker) {
		t.speakerAwaySince = time.Time{}
		return ""
	}
	if t.speakerAwaySince.IsZero() {
		t.speakerAwaySince = now
		t.log.Debugf("speaker %s away: wait for it %s before another node speaks", t.speaker, t.speakerGracePeriod())
	}
	if now.Sub(t.speakerAwaySince) >= t.speakerGracePeriod() {
		return ""
	}
	return t.speaker
}

// speakerMarkFile is the file a speaker stopping leaves, for its collector
// starting again to take the speech back: the nodes wait for it.
func speakerMarkFile() string {
	return filepath.Join(rawconfig.Paths.Var, "node", "collector", "speaker")
}

// markSpeaker leaves the mark of a speaker stopping.
func (t *T) markSpeaker() {
	p := speakerMarkFile()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.log.Warnf("mark the collector speaker: %s", err)
		return
	}
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.log.Warnf("mark the collector speaker: %s", err)
	}
}

// unmarkSpeaker removes the mark of a speaker, as on its speech given up.
func (t *T) unmarkSpeaker() {
	if err := os.Remove(speakerMarkFile()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.log.Warnf("unmark the collector speaker: %s", err)
	}
}

// readSpeakerMark sets wasSpeakerUntil from the mark of a speaker stopping
// less than the speaker grace period ago: the nodes still wait for it.
func (t *T) readSpeakerMark(now time.Time) {
	info, err := os.Stat(speakerMarkFile())
	if err != nil {
		return
	}
	if until := info.ModTime().Add(t.speakerGracePeriod()); now.Before(until) {
		t.wasSpeakerUntil = until
		t.log.Infof("the node was the collector speaker before its stop: take the speech back")
	}
}

// wasSpeaker says whether the node was the speaker before its stop, less
// than the speaker grace period ago: the nodes wait for it to speak again.
func (t *T) wasSpeaker(now time.Time) bool {
	return now.Before(t.wasSpeakerUntil)
}

// stopState returns the state the collector says at its stop: stopping for a
// speaker feeding the collector, which leaves its mark, for the peers to wait
// for it, as with its daemon restarting, and stopped for another node, as the
// node leader speaking with no client or refused credentials, which the peers
// do not wait for.
func (t *T) stopState() string {
	if t.isSpeaker && !t.disable && t.client != nil && !t.credsRefused() {
		t.markSpeaker()
		return "stopping"
	}
	return "stopped"
}

// peersWaitFor says whether no live peer takes another node for the speaker:
// a node restarting takes the speech back from its mark only then, not after
// a peer, waiting for it less than the node did, took the speech.
func (t *T) peersWaitFor() bool {
	for _, nodename := range clusternode.Get() {
		if nodename == t.localhost || node.MonitorData.GetByNode(nodename) == nil {
			continue
		}
		c := daemonsubsystem.DataCollector.Get(nodename)
		if c != nil && c.Speaker != "" && c.Speaker != t.localhost {
			return false
		}
	}
	return true
}

// peersSpeaker returns the speaker the live peers report, other than the node
// itself, "" when none does: the first peer in the cluster nodes reporting a
// live speaker whose state is not known yet, or says it speaks. A speaker
// whose state says it can not speak, or does not speak, is not followed.
func (t *T) peersSpeaker() string {
	for _, nodename := range clusternode.Get() {
		if nodename == t.localhost || node.MonitorData.GetByNode(nodename) == nil {
			continue
		}
		c := daemonsubsystem.DataCollector.Get(nodename)
		if c == nil || c.Speaker == "" || c.Speaker == t.localhost {
			continue
		}
		speaker := c.Speaker
		if speaker != nodename && node.MonitorData.GetByNode(speaker) == nil {
			continue
		}
		if s := daemonsubsystem.DataCollector.Get(speaker); s != nil && s.State != "" && s.State != "speaker" {
			continue
		}
		return speaker
	}
	return ""
}
