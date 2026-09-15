package duration

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type (
	Duration struct {
		time.Duration
	}
)

func New(d time.Duration) *Duration {
	return &Duration{Duration: d}
}

// dayRe matches the day component of a duration.
var dayRe = regexp.MustCompile(`(\d+(?:\.\d+)?)d`)

// Parse reads a duration written the way the agent prints one.
//
// The standard library stops at the hour, because a day is not a fixed number
// of hours everywhere. The agent prints days anyway, in every duration long
// enough to have one, so it has to read them back: a configuration naming "1d"
// is naming what "1d" is shown as, and refusing it would be refusing our own
// output. A day is taken as 24 hours.
func Parse(s string) (time.Duration, error) {
	converted := dayRe.ReplaceAllStringFunc(s, func(match string) string {
		days, err := strconv.ParseFloat(strings.TrimSuffix(match, "d"), 64)
		if err != nil {
			return match
		}
		return strconv.FormatFloat(days*24, 'f', -1, 64) + "h"
	})
	return time.ParseDuration(converted)
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}

	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		dur, err := Parse(s)
		if err != nil {
			return err
		}
		d.Duration = dur
		return nil
	}
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

func (d Duration) IsZero() bool {
	return d.Duration == 0
}

func (d Duration) Positive() bool {
	return d.Duration > 0
}

func FmtShortDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}

	// Below a second no unit below matches, and the loop picking the largest
	// unit that fits would leave the day: a duration of 300ms rendered "0d".
	if d < time.Second {
		if ms := d / time.Millisecond; ms > 0 {
			return strconv.Itoa(int(ms)) + "ms"
		}
		return strconv.Itoa(int(d/time.Microsecond)) + "us"
	}

	day := 24 * time.Hour

	units := []struct {
		duration time.Duration
		suffix   string
	}{
		{day, "d"},
		{time.Hour, "h"},
		{time.Minute, "m"},
		{time.Second, "s"},
	}

	var primaryIdx int
	for i, unit := range units {
		if d >= unit.duration {
			primaryIdx = i
			break
		}
	}

	primary := units[primaryIdx]
	primaryValue := d / primary.duration

	var sb strings.Builder
	sb.WriteString(strconv.Itoa(int(primaryValue)))
	sb.WriteString(primary.suffix)

	if primaryValue < 10 && primaryIdx+1 < len(units) {
		remainder := d % primary.duration
		secondary := units[primaryIdx+1]

		if secondaryValue := remainder / secondary.duration; secondaryValue > 0 {
			sb.WriteString(strconv.Itoa(int(secondaryValue)))
			sb.WriteString(secondary.suffix)
		}
	}

	return sb.String()
}

type (
	// Flag adapts a duration to the command line, so an option accepts what
	// the configuration and the agent output accept, the day unit included.
	Flag struct {
		p *time.Duration
	}
)

// NewFlag returns the command line representation of a duration.
func NewFlag(p *time.Duration) *Flag {
	return &Flag{p: p}
}

func (t *Flag) String() string {
	if t.p == nil || *t.p == 0 {
		return ""
	}
	return t.p.String()
}

func (t *Flag) Set(s string) error {
	d, err := Parse(s)
	if err != nil {
		return err
	}
	*t.p = d
	return nil
}

func (t *Flag) Type() string {
	return "duration"
}
