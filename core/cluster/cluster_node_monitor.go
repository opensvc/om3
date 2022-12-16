package cluster

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type (
	// NodeMonitor describes the in-daemon states of a node
	NodeMonitor struct {
		State               NodeMonitorState        `json:"state"`
		LocalExpect         NodeMonitorLocalExpect  `json:"local_expect"`
		GlobalExpect        NodeMonitorGlobalExpect `json:"global_expect"`
		StateUpdated        time.Time               `json:"state_updated"`
		GlobalExpectUpdated time.Time               `json:"global_expect_updated"`
		LocalExpectUpdated  time.Time               `json:"local_expect_updated"`
	}

	NodeMonitorState        int
	NodeMonitorLocalExpect  int
	NodeMonitorGlobalExpect int
)

const (
	NodeMonitorStateInit NodeMonitorState = iota
	NodeMonitorStateIdle
	NodeMonitorStateDraining
	NodeMonitorStateDrainFailed
	NodeMonitorStateThawedFailed
	NodeMonitorStateFreezeFailed
	NodeMonitorStateFreezing
	NodeMonitorStateFrozen
	NodeMonitorStateThawing
	NodeMonitorStateShutting
	NodeMonitorStateMaintenance
	NodeMonitorStateUpgrade
	NodeMonitorStateRejoin
)

const (
	NodeMonitorLocalExpectUnset NodeMonitorLocalExpect = iota
	NodeMonitorLocalExpectDrained
)

const (
	NodeMonitorGlobalExpectUnset NodeMonitorGlobalExpect = iota
	NodeMonitorGlobalExpectAborted
	NodeMonitorGlobalExpectFrozen
	NodeMonitorGlobalExpectThawed
)

var (
	NodeMonitorStateStrings = map[NodeMonitorState]string{
		NodeMonitorStateDraining:     "draining",
		NodeMonitorStateDrainFailed:  "drain failed",
		NodeMonitorStateIdle:         "idle",
		NodeMonitorStateThawedFailed: "unfreeze failed",
		NodeMonitorStateFreezeFailed: "freeze failed",
		NodeMonitorStateFreezing:     "freezing",
		NodeMonitorStateFrozen:       "frozen",
		NodeMonitorStateThawing:      "thawing",
		NodeMonitorStateShutting:     "shutting",
		NodeMonitorStateMaintenance:  "maintenance",
		NodeMonitorStateInit:         "init",
		NodeMonitorStateUpgrade:      "upgrade",
		NodeMonitorStateRejoin:       "rejoin",
	}

	NodeMonitorStateValues = map[string]NodeMonitorState{
		"draining":        NodeMonitorStateDraining,
		"drain failed":    NodeMonitorStateDrainFailed,
		"idle":            NodeMonitorStateIdle,
		"unfreeze failed": NodeMonitorStateThawedFailed,
		"freeze failed":   NodeMonitorStateFreezeFailed,
		"freezing":        NodeMonitorStateFreezing,
		"frozen":          NodeMonitorStateFrozen,
		"thawing":         NodeMonitorStateThawing,
		"shutting":        NodeMonitorStateShutting,
		"maintenance":     NodeMonitorStateMaintenance,
		"init":            NodeMonitorStateInit,
		"upgrade":         NodeMonitorStateUpgrade,
		"rejoin":          NodeMonitorStateRejoin,
	}

	NodeMonitorLocalExpectStrings = map[NodeMonitorLocalExpect]string{
		NodeMonitorLocalExpectUnset:   "unset",
		NodeMonitorLocalExpectDrained: "drained",
	}

	NodeMonitorLocalExpectValues = map[string]NodeMonitorLocalExpect{
		"unset":   NodeMonitorLocalExpectUnset,
		"drained": NodeMonitorLocalExpectDrained,
	}

	NodeMonitorGlobalExpectStrings = map[NodeMonitorGlobalExpect]string{
		NodeMonitorGlobalExpectAborted: "aborted",
		NodeMonitorGlobalExpectFrozen:  "frozen",
		NodeMonitorGlobalExpectThawed:  "thawed",
		NodeMonitorGlobalExpectUnset:   "unset",
	}

	NodeMonitorGlobalExpectValues = map[string]NodeMonitorGlobalExpect{
		"aborted": NodeMonitorGlobalExpectAborted,
		"frozen":  NodeMonitorGlobalExpectFrozen,
		"thawed":  NodeMonitorGlobalExpectThawed,
		"unset":   NodeMonitorGlobalExpectUnset,
	}

	// the node monitor states evicting a node from ranking algorithms
	NodeMonitorStateUnrankable = map[NodeMonitorState]any{
		NodeMonitorStateMaintenance: nil,
		NodeMonitorStateUpgrade:     nil,
		NodeMonitorStateInit:        nil,
		NodeMonitorStateShutting:    nil,
		NodeMonitorStateRejoin:      nil,
	}
)

func (t NodeMonitorState) IsDoing() bool {
	return strings.HasSuffix(t.String(), "ing")
}

func (t NodeMonitorState) IsRankable() bool {
	_, ok := NodeMonitorStateUnrankable[t]
	return !ok
}

func (n *NodeMonitor) DeepCopy() *NodeMonitor {
	var d NodeMonitor
	d = *n
	return &d
}

func (t NodeMonitorState) String() string {
	return NodeMonitorStateStrings[t]
}

func (t NodeMonitorState) MarshalJSON() ([]byte, error) {
	if s, ok := NodeMonitorStateStrings[t]; !ok {
		fmt.Printf("unexpected NodeMonitorState value: %d\n", t)
		return []byte{}, fmt.Errorf("unexpected NodeMonitorState value: %d", t)
	} else {
		return json.Marshal(s)
	}
}

func (t *NodeMonitorState) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, ok := NodeMonitorStateValues[s]
	if !ok {
		return fmt.Errorf("unexpected NodeMonitorState value: %s", b)
	}
	*t = v
	return nil
}

func (t NodeMonitorLocalExpect) String() string {
	return NodeMonitorLocalExpectStrings[t]
}

func (t NodeMonitorLocalExpect) MarshalJSON() ([]byte, error) {
	if s, ok := NodeMonitorLocalExpectStrings[t]; !ok {
		fmt.Printf("unexpected NodeMonitorLocalExpect value: %d\n", t)
		return []byte{}, fmt.Errorf("unexpected NodeMonitorLocalExpect value: %d", t)
	} else {
		return json.Marshal(s)
	}
}

func (t *NodeMonitorLocalExpect) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, ok := NodeMonitorLocalExpectValues[s]
	if !ok {
		return fmt.Errorf("unexpected NodeMonitorLocalExpect value: %s", b)
	}
	*t = v
	return nil
}

func (t NodeMonitorGlobalExpect) String() string {
	return NodeMonitorGlobalExpectStrings[t]
}

func (t NodeMonitorGlobalExpect) MarshalJSON() ([]byte, error) {
	if s, ok := NodeMonitorGlobalExpectStrings[t]; !ok {
		fmt.Printf("unexpected NodeMonitorGlobalExpect value: %d\n", t)
		return []byte{}, fmt.Errorf("unexpected NodeMonitorGlobalExpect value: %d", t)
	} else {
		return json.Marshal(s)
	}
}

func (t *NodeMonitorGlobalExpect) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, ok := NodeMonitorGlobalExpectValues[s]
	if !ok {
		return fmt.Errorf("unexpected NodeMonitorGlobalExpect value: %s", b)
	}
	*t = v
	return nil
}
