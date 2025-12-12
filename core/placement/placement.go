package placement

import (
	"fmt"

	"github.com/opensvc/om3/v3/util/xmap"
)

type (
	// Policy is an integer representing the opensvc object placement policy.
	Policy int

	// State is optimal when the running <n> instances are placed on the <n> first
	// nodes of the candidate list sorted by the selected placement policy.
	State int
)

const (
	// Invalid is for invalid kinds
	Invalid Policy = iota
	// None is the policy used by special objects like sec, cfg, usr.
	None
	// NodesOrder is the policy where node priorities are assigned to nodes descending from left to right in the nodes list.
	NodesOrder
	// LastStart is the policy where node priorities are assigned to nodes based on instance last start time. The more recent, the higher the priority.
	LastStart
	// LoadAvg is the policy where node priorities are assigned to nodes based on load average. The higher the load, the lower the priority.
	LoadAvg
	// Shift is the policy where node priorities are assigned to nodes based on the scaler slice number.
	Shift
	// Spread is the policy where node priorities are assigned to nodes based on nodename hashing. The node priority is stable as long as the cluster members are stable.
	Spread
	// Score is the policy where node priorities are assigned to nodes based on score. The higher the score, the higher the priority.
	Score
)

const (
	Undef State = iota
	NotApplicable
	Optimal
	NonOptimal
)

var (
	policyToString = map[Policy]string{
		Invalid:    "",
		None:       "none",
		NodesOrder: "nodes order",
		LastStart:  "last start",
		LoadAvg:    "load avg",
		Shift:      "shift",
		Spread:     "spread",
		Score:      "score",
	}

	policyToID = map[string]Policy{
		"":            Invalid,
		"none":        None,
		"nodes order": NodesOrder,
		"last start":  LastStart,
		"load avg":    LoadAvg,
		"shift":       Shift,
		"spread":      Spread,
		"score":       Score,
	}

	stateToString = map[State]string{
		Undef:         "undef",
		NotApplicable: "n/a",
		Optimal:       "optimal",
		NonOptimal:    "non-optimal",
	}

	stateToID = map[string]State{
		"":            Undef,
		"undef":       Undef,
		"n/a":         NotApplicable,
		"optimal":     Optimal,
		"non-optimal": NonOptimal,
	}
)

func (t State) String() string {
	return stateToString[t]
}

// MarshalText marshals the enum as a quoted json string
func (t State) MarshalText() ([]byte, error) {
	if s, ok := stateToString[t]; !ok {
		return nil, fmt.Errorf("unknown placement state %d", t)
	} else {
		return []byte(s), nil
	}
}

// UnmarshalText unmashals a quoted json string to the enum value
func (t *State) UnmarshalText(b []byte) error {
	s := string(b)
	if v, ok := stateToID[s]; !ok {
		return fmt.Errorf("unknown placement state '%s'", s)
	} else {
		*t = v
		return nil
	}
}

func (t Policy) String() string {
	return policyToString[t]
}

// NewPolicy returns a id from its string representation.
func NewPolicy(s string) Policy {
	t, ok := policyToID[s]
	if ok {
		return t
	}
	return Invalid
}

// MarshalText marshals the enum as a quoted json string
func (t Policy) MarshalText() ([]byte, error) {
	if s, ok := policyToString[t]; !ok {
		return nil, fmt.Errorf("unknown placement policy %d", t)
	} else {
		return []byte(s), nil
	}
}

// UnmarshalText unmashals a quoted json string to the enum value
func (t *Policy) UnmarshalText(b []byte) error {
	s := string(b)
	if v, ok := policyToID[s]; !ok {
		return fmt.Errorf("unknown placement policy '%s'", s)
	} else {
		*t = v
		return nil
	}
}

func PolicyNames() []string {
	return xmap.Keys(policyToID)
}
