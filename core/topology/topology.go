package topology

import (
	"fmt"

	"github.com/opensvc/om3/v3/util/xmap"
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
		Invalid:  "invalid",
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

// New returns a topology from its string representation.
func New(s string) T {
	if t, ok := toID[s]; ok {
		return t
	} else {
		return Invalid
	}
}

// MarshalText marshals the enum as a quoted json string
func (t T) MarshalText() ([]byte, error) {
	if s, ok := toString[t]; !ok {
		return nil, fmt.Errorf("unknown topology %d", t)
	} else {
		return []byte(s), nil
	}
}

// UnmarshalText unmashals a quoted json string to the enum value
func (t *T) UnmarshalText(b []byte) error {
	s := string(b)
	v := New(s)
	*t = v
	return nil
}

func Names() []string {
	return xmap.Keys(toID)
}
