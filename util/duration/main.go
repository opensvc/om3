package duration

import (
	"encoding/json"
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

func (d *Duration) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}

	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		dur, err := time.ParseDuration(s)
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
	if d == 0 {
		return "0s"
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
