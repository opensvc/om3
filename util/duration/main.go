package duration

import (
	"encoding/json"
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
