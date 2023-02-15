package node

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type (
	// Monitor describes the in-daemon states of a node
	Monitor struct {
		State               MonitorState        `json:"state"`
		LocalExpect         MonitorLocalExpect  `json:"local_expect"`
		GlobalExpect        MonitorGlobalExpect `json:"global_expect"`
		StateUpdated        time.Time           `json:"state_updated"`
		GlobalExpectUpdated time.Time           `json:"global_expect_updated"`
		LocalExpectUpdated  time.Time           `json:"local_expect_updated"`
	}

	// MonitorUpdate is embedded in the SetNodeMonitor message to
	// change some Monitor values. A nil value does not change the
	// current value.
	MonitorUpdate struct {
		State        *MonitorState        `json:"state"`
		LocalExpect  *MonitorLocalExpect  `json:"local_expect"`
		GlobalExpect *MonitorGlobalExpect `json:"global_expect"`
	}

	MonitorState        int
	MonitorLocalExpect  int
	MonitorGlobalExpect int
)

const (
	MonitorStateZero MonitorState = iota
	MonitorStateIdle
	MonitorStateDraining
	MonitorStateDrainFailed
	MonitorStateDrained
	MonitorStateThawedFailed
	MonitorStateFreezeFailed
	MonitorStateFreezing
	MonitorStateFrozen
	MonitorStateThawing
	MonitorStateShutting
	MonitorStateMaintenance
	MonitorStateUpgrade
	MonitorStateRejoin
)

const (
	MonitorLocalExpectZero MonitorLocalExpect = iota
	MonitorLocalExpectDrained
	MonitorLocalExpectNone
)

const (
	MonitorGlobalExpectZero MonitorGlobalExpect = iota
	MonitorGlobalExpectAborted
	MonitorGlobalExpectFrozen
	MonitorGlobalExpectNone
	MonitorGlobalExpectThawed
)

var (
	MonitorStateStrings = map[MonitorState]string{
		MonitorStateDraining:     "draining",
		MonitorStateDrainFailed:  "drain failed",
		MonitorStateDrained:      "drained",
		MonitorStateIdle:         "idle",
		MonitorStateThawedFailed: "unfreeze failed",
		MonitorStateFreezeFailed: "freeze failed",
		MonitorStateFreezing:     "freezing",
		MonitorStateFrozen:       "frozen",
		MonitorStateThawing:      "thawing",
		MonitorStateShutting:     "shutting",
		MonitorStateMaintenance:  "maintenance",
		MonitorStateZero:         "",
		MonitorStateUpgrade:      "upgrade",
		MonitorStateRejoin:       "rejoin",
	}

	MonitorStateValues = map[string]MonitorState{
		"drained":         MonitorStateDrained,
		"draining":        MonitorStateDraining,
		"drain failed":    MonitorStateDrainFailed,
		"idle":            MonitorStateIdle,
		"unfreeze failed": MonitorStateThawedFailed,
		"freeze failed":   MonitorStateFreezeFailed,
		"freezing":        MonitorStateFreezing,
		"frozen":          MonitorStateFrozen,
		"thawing":         MonitorStateThawing,
		"shutting":        MonitorStateShutting,
		"maintenance":     MonitorStateMaintenance,
		"":                MonitorStateZero,
		"upgrade":         MonitorStateUpgrade,
		"rejoin":          MonitorStateRejoin,
	}

	MonitorLocalExpectStrings = map[MonitorLocalExpect]string{
		MonitorLocalExpectZero:    "",
		MonitorLocalExpectDrained: "drained",
		MonitorLocalExpectNone:    "none",
	}

	MonitorLocalExpectValues = map[string]MonitorLocalExpect{
		"":        MonitorLocalExpectZero,
		"drained": MonitorLocalExpectDrained,
		"none":    MonitorLocalExpectNone,
	}

	MonitorGlobalExpectStrings = map[MonitorGlobalExpect]string{
		MonitorGlobalExpectAborted: "aborted",
		MonitorGlobalExpectFrozen:  "frozen",
		MonitorGlobalExpectNone:    "none",
		MonitorGlobalExpectThawed:  "thawed",
		MonitorGlobalExpectZero:    "",
	}

	MonitorGlobalExpectValues = map[string]MonitorGlobalExpect{
		"aborted": MonitorGlobalExpectAborted,
		"frozen":  MonitorGlobalExpectFrozen,
		"none":    MonitorGlobalExpectNone,
		"thawed":  MonitorGlobalExpectThawed,
		"":        MonitorGlobalExpectZero,
	}

	// the node monitor states evicting a node from ranking algorithms
	MonitorStateUnrankable = map[MonitorState]any{
		MonitorStateMaintenance: nil,
		MonitorStateUpgrade:     nil,
		MonitorStateZero:        nil,
		MonitorStateShutting:    nil,
		MonitorStateRejoin:      nil,
	}
)

func (t MonitorState) IsDoing() bool {
	return strings.HasSuffix(t.String(), "ing")
}

func (t MonitorState) IsRankable() bool {
	_, ok := MonitorStateUnrankable[t]
	return !ok
}

func (n *Monitor) DeepCopy() *Monitor {
	var d Monitor
	d = *n
	return &d
}

func (t MonitorState) String() string {
	return MonitorStateStrings[t]
}

func (t MonitorState) MarshalJSON() ([]byte, error) {
	if s, ok := MonitorStateStrings[t]; !ok {
		fmt.Printf("unexpected node.MonitorState value: %d\n", t)
		return []byte{}, fmt.Errorf("unexpected node.MonitorState value: %d", t)
	} else {
		return json.Marshal(s)
	}
}

func (t *MonitorState) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, ok := MonitorStateValues[s]
	if !ok {
		return fmt.Errorf("unexpected node.MonitorState value: %s", b)
	}
	*t = v
	return nil
}

func (t MonitorLocalExpect) String() string {
	return MonitorLocalExpectStrings[t]
}

func (t MonitorLocalExpect) MarshalJSON() ([]byte, error) {
	if s, ok := MonitorLocalExpectStrings[t]; !ok {
		fmt.Printf("unexpected node.MonitorLocalExpect value: %d\n", t)
		return []byte{}, fmt.Errorf("unexpected node.MonitorLocalExpect value: %d", t)
	} else {
		return json.Marshal(s)
	}
}

func (t *MonitorLocalExpect) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, ok := MonitorLocalExpectValues[s]
	if !ok {
		return fmt.Errorf("unexpected node.MonitorLocalExpect value: %s", b)
	}
	*t = v
	return nil
}

func (t MonitorGlobalExpect) String() string {
	return MonitorGlobalExpectStrings[t]
}

func (t MonitorGlobalExpect) MarshalJSON() ([]byte, error) {
	if s, ok := MonitorGlobalExpectStrings[t]; !ok {
		fmt.Printf("unexpected MonitorGlobalExpect value: %d\n", t)
		return []byte{}, fmt.Errorf("unexpected MonitorGlobalExpect value: %d", t)
	} else {
		return json.Marshal(s)
	}
}

func (t *MonitorGlobalExpect) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, ok := MonitorGlobalExpectValues[s]
	if !ok {
		return fmt.Errorf("unexpected node.MonitorGlobalExpect value: %s", b)
	}
	*t = v
	return nil
}
