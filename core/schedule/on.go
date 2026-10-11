package schedule

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/converters"
)

type (
	// OnFilter is an event an action runs on: its kind, and the values of
	// the fields of the event it requires.
	OnFilter struct {
		Kind   string
		Fields map[string]string
	}

	// OnPace paces the runs the events ask for: a run at most each
	// Interval, stretched up to Max while the events keep coming.
	OnPace struct {
		Interval time.Duration
		Max      time.Duration
	}
)

var (
	// OnKinds are the event kinds an action can run on: the events of the
	// node a file installed from what the node knows is to be installed
	// again on, and the install of files of the object, which what reads
	// them is to read again.
	OnKinds = []string{
		"FileInstalled",
		"NetworkPeersUpdated",
	}

	// DefaultPace is the pace of the runs of an action whose
	// pace is not set.
	DefaultPace = "@10s"
)

// ParseOn returns the filter of the event s says, "<kind>[,<field>=<value>...]".
func ParseOn(s string) (OnFilter, error) {
	l := strings.Split(s, ",")
	f := OnFilter{Kind: l[0], Fields: make(map[string]string)}
	if !slices.Contains(OnKinds, f.Kind) {
		return OnFilter{}, fmt.Errorf("event %s: the kind is not one of %s", s, strings.Join(OnKinds, ", "))
	}
	for _, e := range l[1:] {
		k, v, ok := strings.Cut(e, "=")
		if !ok || k == "" {
			return OnFilter{}, fmt.Errorf("event %s: %s is not <field>=<value>", s, e)
		}
		f.Fields[k] = v
	}
	return f, nil
}

// Match says if the event whose key is key, "<kind>[,<field>=<value>...]",
// is the one the filter says.
func (t OnFilter) Match(key string) bool {
	l := strings.Split(key, ",")
	if l[0] != t.Kind {
		return false
	}
	fields := make(map[string]string)
	for _, e := range l[1:] {
		if k, v, ok := strings.Cut(e, "="); ok {
			fields[k] = v
		}
	}
	for k, v := range t.Fields {
		if fields[k] != v {
			return false
		}
	}
	return true
}

// ParsePace returns the pace s says, "@<interval>[ backoff @<max>]",
// the durations written as the interval of a schedule.
func ParsePace(s string) (OnPace, error) {
	if s == "" {
		s = DefaultPace
	}
	parse := func(word string) (time.Duration, error) {
		v, ok := strings.CutPrefix(word, "@")
		if !ok {
			return 0, fmt.Errorf("pace %s: %s is not @<duration>", s, word)
		}
		if _, err := strconv.Atoi(v); err == nil {
			// no unit, minutes, as in a schedule
			v += "m"
		}
		i, err := converters.Duration.Convert(v)
		if err != nil {
			return 0, fmt.Errorf("pace %s: %w", s, err)
		}
		d := *i.(*time.Duration)
		if d <= 0 {
			return 0, fmt.Errorf("pace %s: %s is not a positive duration", s, word)
		}
		return d, nil
	}
	words := strings.Fields(s)
	switch {
	case len(words) == 1:
	case len(words) == 3 && words[1] == "backoff":
	default:
		return OnPace{}, fmt.Errorf("pace %s: not @<interval>[ backoff @<max>]", s)
	}
	interval, err := parse(words[0])
	if err != nil {
		return OnPace{}, err
	}
	pace := OnPace{Interval: interval, Max: interval}
	if len(words) == 3 {
		if pace.Max, err = parse(words[2]); err != nil {
			return OnPace{}, err
		}
		if pace.Max < pace.Interval {
			return OnPace{}, fmt.Errorf("pace %s: the backoff max is shorter than the interval", s)
		}
	}
	return pace, nil
}
