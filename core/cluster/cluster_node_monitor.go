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
		Status              NodeMonitorStatus       `json:"status"`
		LocalExpect         NodeMonitorLocalExpect  `json:"local_expect"`
		GlobalExpect        NodeMonitorGlobalExpect `json:"global_expect"`
		StatusUpdated       time.Time               `json:"status_updated"`
		GlobalExpectUpdated time.Time               `json:"global_expect_updated"`
		LocalExpectUpdated  time.Time               `json:"local_expect_updated"`
	}

	NodeMonitorStatus       int
	NodeMonitorLocalExpect  int
	NodeMonitorGlobalExpect int
)

const (
	NodeMonitorStatusInit NodeMonitorStatus = iota
	NodeMonitorStatusIdle
	NodeMonitorStatusDraining
	NodeMonitorStatusDrainFailed
	NodeMonitorStatusThawedFailed
	NodeMonitorStatusFreezeFailed
	NodeMonitorStatusFreezing
	NodeMonitorStatusFrozen
	NodeMonitorStatusThawing
	NodeMonitorStatusShutting
	NodeMonitorStatusMaintenance
	NodeMonitorStatusUpgrade
	NodeMonitorStatusRejoin
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
	NodeMonitorStatusStrings = map[NodeMonitorStatus]string{
		NodeMonitorStatusDraining:     "draining",
		NodeMonitorStatusDrainFailed:  "drain failed",
		NodeMonitorStatusIdle:         "idle",
		NodeMonitorStatusThawedFailed: "unfreeze failed",
		NodeMonitorStatusFreezeFailed: "freeze failed",
		NodeMonitorStatusFreezing:     "freezing",
		NodeMonitorStatusFrozen:       "frozen",
		NodeMonitorStatusThawing:      "thawing",
		NodeMonitorStatusShutting:     "shutting",
		NodeMonitorStatusMaintenance:  "maintenance",
		NodeMonitorStatusInit:         "init",
		NodeMonitorStatusUpgrade:      "upgrade",
		NodeMonitorStatusRejoin:       "rejoin",
	}

	NodeMonitorStatusValues = map[string]NodeMonitorStatus{
		"draining":        NodeMonitorStatusDraining,
		"drain failed":    NodeMonitorStatusDrainFailed,
		"idle":            NodeMonitorStatusIdle,
		"unfreeze failed": NodeMonitorStatusThawedFailed,
		"freeze failed":   NodeMonitorStatusFreezeFailed,
		"freezing":        NodeMonitorStatusFreezing,
		"frozen":          NodeMonitorStatusFrozen,
		"thawing":         NodeMonitorStatusThawing,
		"shutting":        NodeMonitorStatusShutting,
		"maintenance":     NodeMonitorStatusMaintenance,
		"init":            NodeMonitorStatusInit,
		"upgrade":         NodeMonitorStatusUpgrade,
		"rejoin":          NodeMonitorStatusRejoin,
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
	NodeMonitorStatusUnrankable = map[NodeMonitorStatus]any{
		NodeMonitorStatusMaintenance: nil,
		NodeMonitorStatusUpgrade:     nil,
		NodeMonitorStatusInit:        nil,
		NodeMonitorStatusShutting:    nil,
		NodeMonitorStatusRejoin:      nil,
	}
)

func (t NodeMonitorStatus) IsDoing() bool {
	return strings.HasSuffix(t.String(), "ing")
}

func (t NodeMonitorStatus) IsRankable() bool {
	_, ok := NodeMonitorStatusUnrankable[t]
	return !ok
}

func (n *NodeMonitor) DeepCopy() *NodeMonitor {
	var d NodeMonitor
	d = *n
	return &d
}

func (t NodeMonitorStatus) String() string {
	return NodeMonitorStatusStrings[t]
}

func (t NodeMonitorStatus) MarshalJSON() ([]byte, error) {
	if s, ok := NodeMonitorStatusStrings[t]; !ok {
		fmt.Printf("unexpected NodeMonitorStatus value: %d\n", t)
		return []byte{}, fmt.Errorf("unexpected NodeMonitorStatus value: %d", t)
	} else {
		return json.Marshal(s)
	}
}

func (t *NodeMonitorStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, ok := NodeMonitorStatusValues[s]
	if !ok {
		return fmt.Errorf("unexpected NodeMonitorStatus value: %s", b)
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
