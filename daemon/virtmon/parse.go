package virtmon

import (
	"strings"
)

// event is a lifecycle event of a libvirt domain, as virsh prints it.
type event struct {
	Domain string
	Event  string
	Detail string
}

// eventPrefix starts the part of a lifecycle event line naming the domain,
// after the timestamp virsh prints with --timestamp.
const eventPrefix = "event 'lifecycle' for domain "

// parseEvent reads a lifecycle event line of "virsh event":
//
//	[<timestamp>: ]event 'lifecycle' for domain '<name>': <Event> <Detail>
//
// The name is quoted by the virsh versions printing it, and may hold spaces
// and colons, so the event is read after the last ": ". Only the name drives
// what follows: a wording of the event a version changes breaks nothing.
func parseEvent(line string) (event, bool) {
	i := strings.Index(line, eventPrefix)
	if i < 0 {
		return event{}, false
	}
	rest := line[i+len(eventPrefix):]
	j := strings.LastIndex(rest, ": ")
	if j < 0 {
		return event{}, false
	}
	name := rest[:j]
	if len(name) >= 2 && strings.HasPrefix(name, "'") && strings.HasSuffix(name, "'") {
		name = name[1 : len(name)-1]
	}
	if name == "" {
		return event{}, false
	}
	ev := event{Domain: name}
	ev.Event, ev.Detail, _ = strings.Cut(strings.TrimSpace(rest[j+2:]), " ")
	return ev, true
}
