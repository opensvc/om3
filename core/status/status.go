package status

import (
	"bytes"
	"encoding/json"
)

// Type representing a Resource, Object Instance or Object status
type Type int

const (
	// Up Configured or Active
	Up Type = iota
	// Down Unconfigured or Inactive
	Down
	// Warn Partially configured or active
	Warn
	// NotApplicable Not Applicable
	NotApplicable
	// Undef Undefined
	Undef
	// StandbyUp Instance with standby resources Configured or Active and no other resources
	StandbyUp
	// StandbyDown Instance with standby resources Unconfigured or Inactive and no other resources
	StandbyDown
	// StandbyUpWithUp Instance with standby resources Configured or Active and other resources Up
	StandbyUpWithUp
	// StandbyUpWithDown Instance with standby resources Configured or Active and other resources Down
	StandbyUpWithDown
)

var toString = map[Type]string{
	Up:                "up",
	Down:              "down",
	Warn:              "warn",
	NotApplicable:     "n/a",
	Undef:             "undef",
	StandbyUp:         "stdby up",
	StandbyDown:       "stdby down",
	StandbyUpWithUp:   "up",
	StandbyUpWithDown: "stdby up",
}

var toID = map[string]Type{
	"up":         Up,
	"down":       Down,
	"warn":       Warn,
	"n/a":        NotApplicable,
	"undef":      Undef,
	"stdby up":   StandbyUp,
	"stdby down": StandbyDown,
}

func (t Type) String() string {
	return toString[t]
}

// MarshalJSON marshals the enum as a quoted json string
func (t Type) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(toString[t])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *Type) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value, 'Created' in this case.
	*t = toID[j]
	return nil
}
