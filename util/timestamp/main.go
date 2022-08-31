// Package timestamp manage Unix timestamps

package timestamp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// T is a unix timestamp with nanosecond precision.
type T struct {
	tm time.Time
}

var (
	zero = time.Unix(0, 0)
)

// New allocates a timestamp from the given time.
func New(tm time.Time) T {
	return T{tm: tm}
}

// NewZero allocates a 0.0 unix timestamp
func NewZero() T {
	return T{tm: time.Unix(0, 0)}
}

// Now return a new timestamp for the present date.
func Now() T {
	return T{tm: time.Now()}
}

// NewFromSecondsFloat64 returns a timestamp instance loaded with the time passed as a float64 seconds since epoch.
func NewFromSecondsFloat64(f float64) T {
	d := time.Duration(f * float64(time.Second))
	tm := time.Unix(0, 0).Add(d)
	return New(tm)
}

func (t T) Time() time.Time {
	return t.tm
}

func (t T) String() string {
	return fmt.Sprintf("%d.%.09d", t.tm.Unix(), t.tm.Nanosecond())
}

// IsZero reports whether t represents the Unix zero time instant,
// January 1, 1970 UTC.
func (t T) IsZero() bool {
	return t.tm.Equal(zero) || t.tm.Before(zero)
}

// MarshalJSON turns this type instance into a byte slice.
func (t T) MarshalJSON() ([]byte, error) {
	if t.tm.IsZero() {
		return json.Marshal(0)
	}
	return []byte(t.String()), nil
}

// UnmarshalJSON parses a byte slice and loads this type instance.
func (t *T) UnmarshalJSON(b []byte) error {
	ti, err := Parse(string(b))
	if err != nil {
		*t = T{}
		return nil
	}
	*t = New(ti)
	return nil
}

func Parse(s string) (time.Time, error) {
	var (
		sec  int64
		nsec int64
	)
	if !strings.Contains(s, ".") {
		s = s + ".0"
	}
	n, err := fmt.Sscanf(s, "%d.%d", &sec, &nsec)
	if err != nil {
		return time.Unix(0, 0), err
	}
	if n != 2 {
		return time.Unix(0, 0), fmt.Errorf("%s: invalid timestamp format: expecting 2 elements separated by a dot", s)
	}
	return time.Unix(sec, nsec), nil
}

func (t T) Render() string {
	layout := "2006-01-02 15:04:05 Z07:00"
	return t.tm.Local().Format(layout)
}
