package topology

import (
	"bytes"
	"encoding/json"

	"opensvc.com/opensvc/util/xmap"
)

// T is an integer representing the opensvc object topology.
type T int

const (
	// Invalid is for invalid kinds
	Invalid T = iota
	// Failover is the topology where only one instance is activable.
	Failover
	// Flex is the topology where many instances are activable simultaneously. At most 1 per node.
	Flex
)

var (
	toString = map[T]string{
		Failover: "failover",
		Flex:     "flex",
	}

	toID = map[string]T{
		"failover": Failover,
		"flex":     Flex,
	}
)

func (t T) String() string {
	return toString[t]
}

// New returns a topogy id from its string representation.
func New(s string) T {
	t, ok := toID[s]
	if ok {
		return t
	}
	return Invalid
}

// MarshalJSON marshals the enum as a quoted json string
func (t T) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(toString[t])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *T) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value, 'Created' in this case.
	*t = toID[j]
	return nil
}

func Names() []string {
	return xmap.Keys(toID)
}
