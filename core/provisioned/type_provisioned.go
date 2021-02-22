package provisioned

import (
	"bytes"
	"encoding/json"
)

// Type stores an integer value representing a service, instance
// or resource provisioned state.
type Type int

const (
	// Undef is used when a resource or instance has no provisioned state.
	Undef Type = iota
	// True means the instance or resource is known to be provisioned.
	True
	// False means the instance or resource is known to be not provisioned.
	False
	// Mixed means the instance or service is partially provisioned.
	Mixed
)

var toString = map[Type]string{
	Undef: "",
	True:  "true",
	False: "false",
	Mixed: "mixed",
}

var sToID = map[string]Type{
	"":      Undef,
	"true":  True,
	"false": False,
	"mixed": Mixed,
}

var bToID = map[bool]Type{
	true:  True,
	false: False,
}

func (t Type) String() string {
	return toString[t]
}

// MarshalJSON marshals the enum as a quoted json string
func (t Type) MarshalJSON() ([]byte, error) {
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

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *Type) UnmarshalJSON(b []byte) error {
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
