package placement

import (
	"bytes"
	"encoding/json"

	"opensvc.com/opensvc/util/xmap"
)

// T is an integer representing the opensvc object placement policy.
type T int

const (
	// Invalid is for invalid kinds
	Invalid T = iota
	// None is the policy used by special objects like sec, cfg, usr.
	None
	// NodesOrder is the policy where node priorities are assigned to nodes descending from left to right in the nodes list.
	NodesOrder
	// LoadAvg is the policy where node priorities are assigned to nodes based on load average. The higher the load, the lower the priority.
	LoadAvg
	// Shift is the policy where node priorities are assigned to nodes based on the scaler slice number.
	Shift
	// Spread is the policy where node priorities are assigned to nodes based on nodename hashing. The node priority is stable as long as the cluster members are stable.
	Spread
	// Score is the policy where node priorities are assigned to nodes based on score. The higher the score, the higher the priority.
	Score
)

var (
	toString = map[T]string{
		None:       "none",
		NodesOrder: "nodes order",
		LoadAvg:    "load avg",
		Shift:      "shift",
		Spread:     "spread",
		Score:      "score",
	}

	toID = map[string]T{
		"none":        None,
		"nodes order": NodesOrder,
		"load avg":    LoadAvg,
		"shift":       Shift,
		"spread":      Spread,
		"score":       Score,
	}
)

func (t T) String() string {
	return toString[t]
}

// New returns a id from its string representation.
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
	*t = toID[j]
	return nil
}

func Names() []string {
	return xmap.Keys(toID)
}
