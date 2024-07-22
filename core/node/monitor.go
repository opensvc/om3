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
		GlobalExpect MonitorGlobalExpect `json:"global_expect"`
		LocalExpect  MonitorLocalExpect  `json:"local_expect"`
		State        MonitorState        `json:"state"`

		GlobalExpectUpdatedAt time.Time `json:"global_expect_updated_at"`
		LocalExpectUpdatedAt  time.Time `json:"local_expect_updated_at"`
		StateUpdatedAt        time.Time `json:"state_updated_at"`
		UpdatedAt             time.Time `json:"updated_at"`

		OrchestrationID     uuid.UUID `json:"orchestration_id"`
		OrchestrationIsDone bool      `json:"orchestration_is_done"`
		SessionID           uuid.UUID `json:"session_id"`

		IsPreserved bool `json:"preserved"`
	}

	// MonitorUpdate is embedded in the SetNodeMonitor message to
	// change some Monitor values. A nil value does not change the
	// current value.
	MonitorUpdate struct {
		State        *MonitorState        `json:"state"`
		LocalExpect  *MonitorLocalExpect  `json:"local_expect"`
		GlobalExpect *MonitorGlobalExpect `json:"global_expect"`

		// CandidateOrchestrationID is a candidate orchestration id for a new imon orchestration.
		CandidateOrchestrationID uuid.UUID `json:"orchestration_id"`
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

	// MonitorStateShutdown is the node monitor state on successfully shutdown
	MonitorStateShutdown

	// MonitorStateShutdownFailed is the node monitor state on failed shutdown
	MonitorStateShutdownFailed

	// MonitorStateShutting is the node monitor state during a shutdown in progress
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
		MonitorStateDraining:       "draining",
		MonitorStateDrainFailed:    "drain failed",
		MonitorStateDrained:        "drained",
		MonitorStateIdle:           "idle",
		MonitorStateThawedFailed:   "unfreeze failed",
		MonitorStateFreezeFailed:   "freeze failed",
		MonitorStateFreezing:       "freezing",
		MonitorStateFrozen:         "frozen",
		MonitorStateThawing:        "thawing",
		MonitorStateShutdown:       "shutdown",
		MonitorStateShutdownFailed: "shutdown failed",
		MonitorStateShutting:       "shutting",
		MonitorStateMaintenance:    "maintenance",
		MonitorStateZero:           "empty",
		MonitorStateUpgrade:        "upgrade",
		MonitorStateRejoin:         "rejoin",
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
		"shutdown":        MonitorStateShutdown,
		"shutdown failed": MonitorStateShutdownFailed,
		"shutting":        MonitorStateShutting,
		"maintenance":     MonitorStateMaintenance,
		"empty":           MonitorStateZero,
		"upgrade":         MonitorStateUpgrade,
		"rejoin":          MonitorStateRejoin,
	}

	MonitorLocalExpectStrings = map[MonitorLocalExpect]string{
		MonitorLocalExpectZero:    "empty",
		MonitorLocalExpectDrained: "drained",
		MonitorLocalExpectNone:    "none",
	}

	MonitorLocalExpectValues = map[string]MonitorLocalExpect{
		"empty":   MonitorLocalExpectZero,
		"drained": MonitorLocalExpectDrained,
		"none":    MonitorLocalExpectNone,
	}

	MonitorGlobalExpectStrings = map[MonitorGlobalExpect]string{
		MonitorGlobalExpectAborted: "aborted",
		MonitorGlobalExpectFrozen:  "frozen",
		MonitorGlobalExpectNone:    "none",
		MonitorGlobalExpectThawed:  "thawed",
		MonitorGlobalExpectZero:    "empty",
	}

	MonitorGlobalExpectValues = map[string]MonitorGlobalExpect{
		"aborted": MonitorGlobalExpectAborted,
		"frozen":  MonitorGlobalExpectFrozen,
		"none":    MonitorGlobalExpectNone,
		"thawed":  MonitorGlobalExpectThawed,
		"empty":   MonitorGlobalExpectZero,
	}

	// MonitorStateUnrankable is the node monitor states evicting a node from ranking algorithms
	MonitorStateUnrankable = map[MonitorState]any{
		MonitorStateMaintenance:    nil,
		MonitorStateUpgrade:        nil,
		MonitorStateZero:           nil,
		MonitorStateShutdown:       nil,
		MonitorStateShutdownFailed: nil,
		MonitorStateShutting:       nil,
		MonitorStateRejoin:         nil,
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

func (t *Monitor) Unstructured() map[string]any {
	return map[string]any{
		"global_expect":            t.GlobalExpect,
		"local_expect":             t.LocalExpect,
		"state":                    t.State,
		"global_expect_updated_at": t.GlobalExpectUpdatedAt,
		"local_expect_updated_at":  t.LocalExpectUpdatedAt,
		"state_updated_at":         t.StateUpdatedAt,
		"updated_at":               t.UpdatedAt,
		"orchestration_id":         t.OrchestrationID,
		"orchestration_is_done":    t.OrchestrationIsDone,
		"session_id":               t.SessionID,
	}
}

func (t MonitorUpdate) String() string {
	s := fmt.Sprintf("CandidateOrchestrationID=%s", t.CandidateOrchestrationID)
	if t.State != nil {
		s += fmt.Sprintf(" State=%s", t.State)
	}
	if t.LocalExpect != nil {
		s += fmt.Sprintf(" LocalExpect=%s", t.LocalExpect)
	}
	if t.GlobalExpect != nil {
		s += fmt.Sprintf(" GlobalExpect=%s", t.GlobalExpect)
	}
	return fmt.Sprintf("node.MonitorUpdate{%s}", s)
}
