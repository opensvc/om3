package status

import (
	"bytes"
	"encoding/json"

	"github.com/fatih/color"
)

// Type representing a Resource, Object Instance or Object status
type Type int

const (
	// NotApplicable means Not Applicable
	NotApplicable Type = iota
	// Up means Configured or Active
	Up
	// Down means Unconfigured or Inactive
	Down
	// Warn means Partially configured or active
	Warn
	// Undef means Undefined
	Undef
	// StandbyUp means Instance with standby resources Configured or Active and no other resources
	StandbyUp
	// StandbyDown means Instance with standby resources Unconfigured or Inactive and no other resources
	StandbyDown
	// StandbyUpWithUp means Instance with standby resources Configured or Active and other resources Up
	StandbyUpWithUp
	// StandbyUpWithDown means Instance with standby resources Configured or Active and other resources Down
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

var toColor = map[Type]color.Attribute{
	Up:                color.FgGreen,
	Down:              color.FgRed,
	Warn:              color.FgYellow,
	NotApplicable:     color.FgHiBlack,
	Undef:             color.FgHiBlack,
	StandbyUp:         color.FgGreen,
	StandbyDown:       color.FgRed,
	StandbyUpWithUp:   color.FgGreen,
	StandbyUpWithDown: color.FgGreen,
}

func (t Type) String() string {
	return toString[t]
}

// ColorString returns a colorized string representation of the status.
func (t Type) ColorString() string {
	c := toColor[t]
	f := color.New(c).SprintfFunc()
	s := t.String()
	return f(s)
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
