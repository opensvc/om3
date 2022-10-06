package provisioned

import (
	"encoding/json"
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

func (t T) Bool() bool {
	switch t {
	case True:
		return true
	default:
		return false
	}
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

// MarshalJSON marshals the enum as a quoted json string
func (t T) MarshalJSON() (b []byte, err error) {
	v, ok := toString[t]
	if ok {
		return json.Marshal(v)
	}
	err = fmt.Errorf("MarshalJSON unexpected provisioned.T value %d", t)
	return
}

// UnmarshalJSON unmarshals a quoted json string to the enum value
func (t *T) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	v, ok := sToID[s]
	if !ok {
		return fmt.Errorf("unexpected provisioned value: %s", b)
	}
	*t = v
	return nil
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
