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
	return "", fmt.Errorf("unknown heartbeat: %s%s", name, expectedHeartbeatNames(names, false))
}

// heartbeatStreamName returns the component name of the heartbeat stream an
// action addresses, "hb#1.rx" for "1.rx".
//
// The action is published as a message the heartbeat janitor subscribes to by
// component name, so a name no stream answers to is accepted, queued and acted
// on by nobody: "1.rxx" was a no-op the caller was told nothing about.
func heartbeatStreamName(name api.InPathHeartbeatName) (string, error) {
	names, err := configuredHeartbeatNames()
	if err != nil {
		return "", err
	}
	stream := sectionOfHeartbeatName(name)
	for _, section := range names {
		for _, suffix := range heartbeatStreamSuffixes {
			if stream == section+"."+suffix {
				return stream, nil
			}
		}
	}
	return "", fmt.Errorf("unknown heartbeat stream: %s%s", name, expectedHeartbeatNames(names, true))
}

// sectionOfHeartbeatName returns the name as the configuration spells it,
// whether the caller typed the "hb#" prefix or not.
func sectionOfHeartbeatName(name api.InPathHeartbeatName) string {
	return "hb#" + strings.TrimPrefix(string(name), "hb#")
}

// expectedHeartbeatNames renders the tail of the error a refused name is
// reported with, naming what the node would have accepted.
func expectedHeartbeatNames(names []string, streams bool) string {
	l := make([]string, 0, len(names)*2)
	for _, section := range names {
		short := strings.TrimPrefix(section, "hb#")
		if !streams {
			l = append(l, short)
			continue
		}
		for _, suffix := range heartbeatStreamSuffixes {
			l = append(l, short+"."+suffix)
		}
	}
	if len(l) == 0 {
		return ", and this node configures no heartbeat"
	}
	return ", expected one of " + strings.Join(l, ", ")
}
