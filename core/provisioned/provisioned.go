package provisioned

import (
	"bytes"
	"encoding/json"
)

// T stores an integer value representing a service, instance
// or resource provisioned state.
type T int

const (
	// Undef is used when a resource or instance has no provisioned state.
	Undef T = 0
)

const (
	// True means the instance or resource is known to be provisioned.
	True T = 1 << iota
	// False means the instance or resource is known to be not provisioned.
	False
	// Mixed means the instance or service is partially provisioned.
	Mixed
	// NotApplicable means the resource does not need provisioning.
	NotApplicable
)

var toString = map[T]string{
	Undef:         "",
	True:          "true",
	False:         "false",
	Mixed:         "mixed",
	NotApplicable: "n/a",
}

var sToID = map[string]T{
	"":      Undef,
	"true":  True,
	"false": False,
	"mixed": Mixed,
	"n/a":   NotApplicable,
}

var bToID = map[bool]T{
	true:  True,
	false: False,
}

func (t T) String() string {
	return toString[t]
}

//
// FlagString returns a one character representation of the type instance.
//
//   .  Provisioned
//   P  Not provisioned
//   p  Mixed provisioned
//   /  Not applicable
//   ?  Unknown
//
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
func (t T) MarshalJSON() ([]byte, error) {
	var buffer *bytes.Buffer
	switch t {
	case True, False:
		buffer = bytes.NewBufferString(``)
		buffer.WriteString(toString[t])
	default:
		buffer = bytes.NewBufferString(`"`)
		buffer.WriteString(toString[t])
		buffer.WriteString(`"`)
	}
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted json string to the enum value
func (t *T) UnmarshalJSON(b []byte) error {
	var j interface{}
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	switch j.(type) {
	case string:
		*t = sToID[j.(string)]
	case bool:
		*t = bToID[j.(bool)]
	}
	// Note that if the string cannot be found then it will be set to the zero value.
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
	switch t | o {
	case True | True:
		return True
	case True | False:
		return Mixed
	case True | Mixed:
		return Mixed
	case False | Mixed:
		return Mixed
	default:
		return Undef
	}
}
