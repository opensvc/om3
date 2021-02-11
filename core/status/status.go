package status

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

func (t Type) String() string {
	switch t {
	case Up:
		return "up"
	case Down:
		return "down"
	case Warn:
		return "warn"
	case NotApplicable:
		return "n/a"
	case Undef:
		return "undef"
	case StandbyUp:
		return "stdby up"
	case StandbyDown:
		return "stdby down"
	case StandbyUpWithUp:
		return "up"
	case StandbyUpWithDown:
		return "stdby up"
	default:
		return "unknown"
	}
}
