package daemonapi

import (
	"fmt"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/clusterhb"
	"github.com/opensvc/om3/v3/daemon/api"
)

// heartbeatStreamSuffixes are the two directions a heartbeat runs in, each a
// component of its own, started and stopped on its own.
var heartbeatStreamSuffixes = []string{"rx", "tx"}

// configuredHeartbeatNames returns the heartbeat section names the merged node
// and cluster configuration defines, as "hb#1".
//
// It is a variable so a test can pin the names a node has no configuration to
// hold.
var configuredHeartbeatNames = func() ([]string, error) {
	n, err := clusterhb.New()
	if err != nil {
		return nil, err
	}
	return n.HbNames(), nil
}

// heartbeatName returns the configuration section of the heartbeat an action
// addresses, "hb#1" for "1".
//
// The "hb#" prefix "om daemon hb status" shows in a stream id is accepted too,
// so a name read there can be typed back.
func heartbeatName(name api.InPathHeartbeatName) (string, error) {
	names, err := configuredHeartbeatNames()
	if err != nil {
		return "", err
	}
	section := sectionOfHeartbeatName(name)
	if slices.Contains(names, section) {
		return section, nil
	}
	return "", fmt.Errorf("unknown heartbeat: %s%s", name, expectedHeartbeatNames(names))
}

// heartbeatStreamNames returns the component names of the heartbeat streams an
// action addresses: the one a stream name designates, "hb#1.rx" for "1.rx", or
// both of the ones a heartbeat name does, "hb#1.rx" and "hb#1.tx" for "1".
//
// The two streams of a heartbeat run and stop on their own, which is why they
// are addressed on their own. They are one thing to a caller naming the
// heartbeat, who had to name both to say so.
//
// The action is published as a message the heartbeat janitor subscribes to by
// component name, so a name no stream answers to is accepted, queued and acted
// on by nobody: "1.rxx" was a no-op the caller was told nothing about.
func heartbeatStreamNames(name api.InPathHeartbeatName) ([]string, error) {
	names, err := configuredHeartbeatNames()
	if err != nil {
		return nil, err
	}
	stream := sectionOfHeartbeatName(name)
	for _, section := range names {
		if stream == section {
			l := make([]string, 0, len(heartbeatStreamSuffixes))
			for _, suffix := range heartbeatStreamSuffixes {
				l = append(l, section+"."+suffix)
			}
			return l, nil
		}
		for _, suffix := range heartbeatStreamSuffixes {
			if stream == section+"."+suffix {
				return []string{stream}, nil
			}
		}
	}
	return nil, fmt.Errorf("unknown heartbeat stream: %s%s", name, expectedHeartbeatStreamNames(names))
}

// sectionOfHeartbeatName returns the name as the configuration spells it,
// whether the caller typed the "hb#" prefix or not.
func sectionOfHeartbeatName(name api.InPathHeartbeatName) string {
	return "hb#" + strings.TrimPrefix(string(name), "hb#")
}

// expectedHeartbeatNames renders the tail of the error a refused heartbeat
// name is reported with, naming the heartbeats the node would have accepted.
func expectedHeartbeatNames(names []string) string {
	l := shortHeartbeatNames(names)
	if len(l) == 0 {
		return noHeartbeatConfigured
	}
	return ", expected one of " + strings.Join(l, ", ")
}

// expectedHeartbeatStreamNames renders the same tail for a refused stream
// name, naming the streams and then the heartbeats, which stand for both of
// their streams.
func expectedHeartbeatStreamNames(names []string) string {
	short := shortHeartbeatNames(names)
	if len(short) == 0 {
		return noHeartbeatConfigured
	}
	l := make([]string, 0, len(short)*len(heartbeatStreamSuffixes))
	for _, name := range short {
		for _, suffix := range heartbeatStreamSuffixes {
			l = append(l, name+"."+suffix)
		}
	}
	return ", expected one of " + strings.Join(l, ", ") +
		", or " + strings.Join(short, ", ") + " for both streams of a heartbeat"
}

// noHeartbeatConfigured is the tail of the error a name is refused with when
// no name at all would have been accepted.
const noHeartbeatConfigured = ", and this node configures no heartbeat"

// shortHeartbeatNames renders the section names as the api takes them, "1"
// for "hb#1".
func shortHeartbeatNames(names []string) []string {
	l := make([]string, 0, len(names))
	for _, section := range names {
		l = append(l, strings.TrimPrefix(section, "hb#"))
	}
	return l
}
