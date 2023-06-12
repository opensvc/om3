package provisioned

import (
	"errors"
	"fmt"
)

// T stores an integer value representing a service, instance
// or resource provisioned state.
type T int

const (
	// Undef is used when a resource or instance has no provisioned state.
	Undef T = iota
	// True means the instance or resource is known to be provisioned.
	True
	// False means the instance or resource is known to be not provisioned.
	False
	// Mixed means the instance or service is partially provisioned.
	Mixed
	// NotApplicable means the resource does not need provisioning.
	NotApplicable
)

var toString = map[T]string{
	Undef:         "undef",
	True:          "true",
	False:         "false",
	Mixed:         "mixed",
	NotApplicable: "n/a",
}

var sToID = map[string]T{
	"undef": Undef,
	"true":  True,
	"false": False,
	"mixed": Mixed,
	"n/a":   NotApplicable,
}

var bToID = map[bool]T{
	true:  True,
	false: False,
}

// NewFromString return new T from a string representation of T
func NewFromString(s string) (T, error) {
	t, ok := sToID[s]
	if ok {
		return t, nil
	}
	return Undef, errors.New("invalid provisioned string: " + s)
}

// FromBool is a factory function resource drivers can use to return a
// provisioned.T from a boolean
func FromBool(v bool) T {
	return bToID[v]
}

func (t T) String() string {
	return toString[t]
}

func (t T) IsNoneOf(states ...T) bool {
	for _, s := range states {
		if t == s {
			return false
		}
	}
	return true
}

func (t T) IsOneOf(states ...T) bool {
	for _, s := range states {
		if t == s {
			return true
		}
	}
	return false
}

// FlagString returns a one character representation of the type instance.
//
//	.  Provisioned
//	P  Not provisioned
//	p  Mixed provisioned
//	/  Not applicable
//	?  Unknown
func (t T) FlagString() string {
	switch t {
	case True:
		return "."
	case False:
		return "P"
	case Mixed:
		return "p"
	case NotApplicable:
		return "/"
	case Undef:
		return "/"
	default:
		return "?"
	}
}

// MarshalText marshals the enum as a quoted json string
func (t T) MarshalText() ([]byte, error) {
	if s, ok := toString[t]; !ok {
		return nil, fmt.Errorf("unexpected provisioned.T value %d", s)
	} else {
		return []byte(s), nil
	}
}

// UnmarshalText unmarshals a quoted json string to the enum value
func (t *T) UnmarshalText(b []byte) error {
	s := string(b)
	if v, ok := sToID[s]; !ok {
		return fmt.Errorf("unexpected provisioned value: %s", s)
	} else {
		*t = v
		return nil
	}
}

func (t *T) Add(o T) {
	*t = t.And(o)
}

// And merges two states and returns the aggregate state.
func (t T) And(o T) T {
	// handle invariants
	switch t {
	case Undef, NotApplicable:
		return o
	}
	switch o {
	case Undef, NotApplicable:
		return t
	}
	// other merges
	if t != o {
		return Mixed
	}
	return o
}
