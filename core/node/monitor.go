package node

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type (
	// Monitor describes the in-daemon states of a node
	Monitor struct {
		GlobalExpect MonitorGlobalExpect `json:"global_expect" yaml:"global_expect"`
		LocalExpect  MonitorLocalExpect  `json:"local_expect" yaml:"local_expect"`
		State        MonitorState        `json:"state" yaml:"state"`

		GlobalExpectUpdatedAt time.Time `json:"global_expect_updated_at" yaml:"global_expect_updated_at"`
		LocalExpectUpdatedAt  time.Time `json:"local_expect_updated_at" yaml:"local_expect_updated_at"`
		StateUpdatedAt        time.Time `json:"state_updated_at" yaml:"state_updated_at"`
		UpdatedAt             time.Time `json:"updated_at" yaml:"updated_at"`

		OrchestrationId     uuid.UUID `json:"orchestration_id" yaml:"orchestration_id"`
		OrchestrationIsDone bool      `json:"orchestration_is_done" yaml:"orchestration_is_done"`
		SessionId           uuid.UUID `json:"session_id" yaml:"session_id"`
	}

	// MonitorUpdate is embedded in the SetNodeMonitor message to
	// change some Monitor values. A nil value does not change the
	// current value.
	MonitorUpdate struct {
		State        *MonitorState        `json:"state" yaml:"state"`
		LocalExpect  *MonitorLocalExpect  `json:"local_expect" yaml:"local_expect"`
		GlobalExpect *MonitorGlobalExpect `json:"global_expect" yaml:"global_expect"`

		// CandidateOrchestrationId is a candidate orchestration id for a new imon orchestration.
		CandidateOrchestrationId uuid.UUID `json:"orchestration_id" yaml:"orchestration_id"`
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

	ErrInvalidGlobalExpect = errors.New("invalid node monitor global expect")
	ErrInvalidLocalExpect  = errors.New("invalid node monitor local expect")
	ErrInvalidState        = errors.New("invalid node monitor state")
	ErrSameGlobalExpect    = errors.New("node monitor global expect is already set to the same value")
	ErrSameLocalExpect     = errors.New("node monitor local expect is already set to the same value")
	ErrSameState           = errors.New("node monitor state is already set to the same value")
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

func (t MonitorState) MarshalText() ([]byte, error) {
	if s, ok := MonitorStateStrings[t]; !ok {
		return nil, fmt.Errorf("unexpected node.MonitorState value: %d", t)
	} else {
		return []byte(s), nil
	}
}

func (t *MonitorState) UnmarshalText(b []byte) error {
	s := string(b)
	if v, ok := MonitorStateValues[s]; !ok {
		return fmt.Errorf("unexpected node.MonitorState value: %s", b)
	} else {
		*t = v
		return nil
	}
}

func (t MonitorLocalExpect) String() string {
	return MonitorLocalExpectStrings[t]
}

func (t MonitorLocalExpect) MarshalText() ([]byte, error) {
	if s, ok := MonitorLocalExpectStrings[t]; !ok {
		return nil, fmt.Errorf("unexpected node.MonitorLocalExpect value: %d", t)
	} else {
		return []byte(s), nil
	}
}

func (t *MonitorLocalExpect) UnmarshalText(b []byte) error {
	s := string(b)
	if v, ok := MonitorLocalExpectValues[s]; !ok {
		return fmt.Errorf("unexpected node.MonitorLocalExpect value: %s", b)
	} else {
		*t = v
		return nil
	}
}

func (t MonitorGlobalExpect) String() string {
	return MonitorGlobalExpectStrings[t]
}

func (t MonitorGlobalExpect) MarshalText() ([]byte, error) {
	if s, ok := MonitorGlobalExpectStrings[t]; !ok {
		return nil, fmt.Errorf("unexpected MonitorGlobalExpect value: %d", t)
	} else {
		return []byte(s), nil
	}
}

func (t *MonitorGlobalExpect) UnmarshalText(b []byte) error {
	s := string(b)
	if v, ok := MonitorGlobalExpectValues[s]; !ok {
		return fmt.Errorf("unexpected node.MonitorGlobalExpect value: %s", b)
	} else {
		*t = v
		return nil
	}
}
