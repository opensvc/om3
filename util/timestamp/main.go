package timestamp

import (
	"encoding/json"
	"fmt"
	"time"
)

// T is a unix timestamp with nanosecond precision.
type T struct {
	tm time.Time
}

// New allocates a timestamp from the given time.
func New(tm time.Time) T {
	return T{tm: tm}
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

func (t T) String() string {
	return fmt.Sprintf("%d.%d", t.tm.Unix(), t.tm.Nanosecond())
}

// IsZero relays time.Time IsZero.
func (t T) IsZero() bool {
	return t.tm.IsZero()
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
	var (
		sec  int64
		nsec int64
	)
	if n, err := fmt.Sscanf(string(b), "%d.%d", &sec, &nsec); err != nil || n != 2 {
		*t = T{}
		return nil
	}
	*t = New(time.Unix(sec, nsec))
	return nil
}
