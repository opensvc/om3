package instance

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type (
	// Monitor describes the in-daemon states of an instance
	Monitor struct {
		GlobalExpect            MonitorGlobalExpect `json:"global_expect"`
		GlobalExpectUpdated     time.Time           `json:"global_expect_updated"`
		GlobalExpectOptions     any                 `json:"global_expect_options"`
		IsLeader                bool                `json:"is_leader"`
		IsHALeader              bool                `json:"is_ha_leader"`
		LocalExpect             MonitorLocalExpect  `json:"local_expect"`
		LocalExpectUpdated      time.Time           `json:"local_expect_updated"`
		SessionId               string              `json:"session_id"`
		State                   MonitorState        `json:"state"`
		StateUpdated            time.Time           `json:"state_updated"`
		MonitorActionExecutedAt time.Time           `json:"monitor_action_executed_at"`
		Resources               ResourceMonitorMap  `json:"resources,omitempty"`
		UpdatedAt               time.Time           `json:"updated_at"`
	}

	ResourceMonitorMap map[string]ResourceMonitor

	// MonitorUpdate is embedded in the SetInstanceMonitor message to
	// change some Monitor values. A nil value does not change the
	// current value.
	MonitorUpdate struct {
		GlobalExpect        *MonitorGlobalExpect `json:"global_expect"`
		GlobalExpectOptions any                  `json:"global_expect_options"`
		LocalExpect         *MonitorLocalExpect  `json:"local_expect"`
		State               *MonitorState        `json:"state"`
	}

	// ResourceMonitor describes the restart states maintained by the daemon
	// for an object instance.
	ResourceMonitor struct {
		Restart ResourceMonitorRestart `json:"restart"`
	}
	ResourceMonitorRestart struct {
		Remaining int         `json:"remaining"`
		LastAt    time.Time   `json:"last_at"`
		Timer     *time.Timer `json:"-"`
	}

	MonitorState        int
	MonitorLocalExpect  int
	MonitorGlobalExpect int

	MonitorGlobalExpectOptionsPlacedAt struct {
		Destination []string `json:"destination"`
	}
)

const (
	MonitorStateZero MonitorState = iota
	MonitorStateIdle
	MonitorStateDeleted
	MonitorStateDeleting
	MonitorStateFreezeFailed
	MonitorStateFreezing
	MonitorStateFrozen
	MonitorStateProvisioned
	MonitorStateProvisioning
	MonitorStateProvisionFailed
	MonitorStatePurgeFailed
	MonitorStateReady
	MonitorStateShutting
	MonitorStateStarted
	MonitorStateStartFailed
	MonitorStateStarting
	MonitorStateStopFailed
	MonitorStateStopped
	MonitorStateStopping
	MonitorStateThawed
	MonitorStateThawedFailed
	MonitorStateThawing
	MonitorStateUnprovisioned
	MonitorStateUnprovisionFailed
	MonitorStateUnprovisioning
	MonitorStateWaitLeader
	MonitorStateWaitNonLeader
)

const (
	MonitorLocalExpectZero MonitorLocalExpect = iota
	MonitorLocalExpectNone
	MonitorLocalExpectStarted
)

const (
	MonitorGlobalExpectZero MonitorGlobalExpect = iota
	MonitorGlobalExpectAborted
	MonitorGlobalExpectFrozen
	MonitorGlobalExpectNone
	MonitorGlobalExpectPlaced
	MonitorGlobalExpectPlacedAt
	MonitorGlobalExpectProvisioned
	MonitorGlobalExpectPurged
	MonitorGlobalExpectStarted
	MonitorGlobalExpectStopped
	MonitorGlobalExpectThawed
	MonitorGlobalExpectUnprovisioned
)

var (
	MonitorStateStrings = map[MonitorState]string{
		MonitorStateDeleted:           "deleted",
		MonitorStateDeleting:          "deleting",
		MonitorStateFreezeFailed:      "freeze failed",
		MonitorStateFreezing:          "freezing",
		MonitorStateFrozen:            "frozen",
		MonitorStateIdle:              "idle",
		MonitorStateProvisioned:       "provisioned",
		MonitorStateProvisioning:      "provisioning",
		MonitorStateProvisionFailed:   "provision failed",
		MonitorStatePurgeFailed:       "purge failed",
		MonitorStateReady:             "ready",
		MonitorStateShutting:          "shutting",
		MonitorStateStarted:           "started",
		MonitorStateStartFailed:       "start failed",
		MonitorStateStarting:          "starting",
		MonitorStateStopFailed:        "stop failed",
		MonitorStateStopped:           "stopped",
		MonitorStateStopping:          "stopping",
		MonitorStateThawed:            "thawed",
		MonitorStateThawedFailed:      "unfreeze failed",
		MonitorStateThawing:           "thawing",
		MonitorStateUnprovisioned:     "unprovisioned",
		MonitorStateUnprovisionFailed: "unprovision failed",
		MonitorStateUnprovisioning:    "unprovisioning",
		MonitorStateWaitLeader:        "wait leader",
		MonitorStateWaitNonLeader:     "wait non-leader",
		MonitorStateZero:              "",
	}

	MonitorStateValues = map[string]MonitorState{
		"":                   MonitorStateZero,
		"idle":               MonitorStateIdle,
		"deleted":            MonitorStateDeleted,
		"deleting":           MonitorStateDeleting,
		"freeze failed":      MonitorStateFreezeFailed,
		"freezing":           MonitorStateFreezing,
		"frozen":             MonitorStateFrozen,
		"provisioned":        MonitorStateProvisioned,
		"provisioning":       MonitorStateProvisioning,
		"provision failed":   MonitorStateProvisionFailed,
		"purge failed":       MonitorStatePurgeFailed,
		"ready":              MonitorStateReady,
		"shutting":           MonitorStateShutting,
		"started":            MonitorStateStarted,
		"start failed":       MonitorStateStartFailed,
		"starting":           MonitorStateStarting,
		"stop failed":        MonitorStateStopFailed,
		"stopped":            MonitorStateStopped,
		"stopping":           MonitorStateStopping,
		"thawed":             MonitorStateThawed,
		"unfreeze failed":    MonitorStateThawedFailed,
		"thawing":            MonitorStateThawing,
		"unprovisioned":      MonitorStateUnprovisioned,
		"unprovision failed": MonitorStateUnprovisionFailed,
		"unprovisioning":     MonitorStateUnprovisioning,
		"wait leader":        MonitorStateWaitLeader,
		"wait non-leader":    MonitorStateWaitNonLeader,
	}

	MonitorLocalExpectStrings = map[MonitorLocalExpect]string{
		MonitorLocalExpectStarted: "started",
		MonitorLocalExpectNone:    "none",
		MonitorLocalExpectZero:    "",
	}

	MonitorLocalExpectValues = map[string]MonitorLocalExpect{
		"started": MonitorLocalExpectStarted,
		"none":    MonitorLocalExpectNone,
		"":        MonitorLocalExpectZero,
	}

	MonitorGlobalExpectStrings = map[MonitorGlobalExpect]string{
		MonitorGlobalExpectAborted:       "aborted",
		MonitorGlobalExpectZero:          "",
		MonitorGlobalExpectFrozen:        "frozen",
		MonitorGlobalExpectNone:          "none",
		MonitorGlobalExpectPlaced:        "placed",
		MonitorGlobalExpectPlacedAt:      "placed@",
		MonitorGlobalExpectProvisioned:   "provisioned",
		MonitorGlobalExpectPurged:        "purged",
		MonitorGlobalExpectStarted:       "started",
		MonitorGlobalExpectStopped:       "stopped",
		MonitorGlobalExpectThawed:        "thawed",
		MonitorGlobalExpectUnprovisioned: "unprovisioned",
	}

	MonitorGlobalExpectValues = map[string]MonitorGlobalExpect{
		"aborted":       MonitorGlobalExpectAborted,
		"":              MonitorGlobalExpectZero,
		"frozen":        MonitorGlobalExpectFrozen,
		"placed":        MonitorGlobalExpectPlaced,
		"placed@":       MonitorGlobalExpectPlacedAt,
		"provisioned":   MonitorGlobalExpectProvisioned,
		"purged":        MonitorGlobalExpectPurged,
		"started":       MonitorGlobalExpectStarted,
		"stopped":       MonitorGlobalExpectStopped,
		"thawed":        MonitorGlobalExpectThawed,
		"unprovisioned": MonitorGlobalExpectUnprovisioned,
		"none":          MonitorGlobalExpectNone,
	}
)

func (t MonitorState) IsDoing() bool {
	return strings.HasSuffix(t.String(), "ing")
}

func (t MonitorState) String() string {
	return MonitorStateStrings[t]
}

func (t MonitorState) MarshalJSON() ([]byte, error) {
	if s, ok := MonitorStateStrings[t]; !ok {
		fmt.Printf("unexpected MonitorState value: %d\n", t)
		return []byte{}, fmt.Errorf("unexpected MonitorState value: %d", t)
	} else {
		return json.Marshal(s)
	}
}

func (t *Monitor) UnmarshalJSON(b []byte) error {
	type tempMonitor Monitor
	var mon tempMonitor
	if err := json.Unmarshal(b, &mon); err != nil {
		return err
	}
	switch mon.GlobalExpect {
	case MonitorGlobalExpectPlacedAt:
		var options MonitorGlobalExpectOptionsPlacedAt
		if b, err := json.Marshal(mon.GlobalExpectOptions); err != nil {
			return err
		} else if err := json.Unmarshal(b, &options); err != nil {
			return err
		} else {
			mon.GlobalExpectOptions = options
		}
	}
	*t = Monitor(mon)
	return nil
}

func (t *MonitorState) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, ok := MonitorStateValues[s]
	if !ok {
		return fmt.Errorf("unexpected MonitorState value: %s", b)
	}
	*t = v
	return nil
}

func (t MonitorLocalExpect) String() string {
	return MonitorLocalExpectStrings[t]
}

func (t MonitorLocalExpect) MarshalJSON() ([]byte, error) {
	if s, ok := MonitorLocalExpectStrings[t]; !ok {
		fmt.Printf("unexpected MonitorLocalExpect value: %d\n", t)
		return []byte{}, fmt.Errorf("unexpected MonitorLocalExpect value: %d", t)
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
		return fmt.Errorf("unexpected MonitorLocalExpect value: %s", b)
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
		return fmt.Errorf("unexpected MonitorGlobalExpect value: %s", b)
	}
	*t = v
	return nil
}

func (m ResourceMonitorMap) DecRestartRemaining(rid string) {
	if rmon, ok := m[rid]; ok && rmon.Restart.Remaining > 0 {
		rmon.Restart.Remaining -= 1
		m[rid] = rmon
	}
}

func (m ResourceMonitorMap) GetRestartRemaining(rid string) (int, bool) {
	if rmon, ok := m[rid]; ok {
		return rmon.Restart.Remaining, true
	} else {
		return 0, false
	}
}

func (m ResourceMonitorMap) GetRestartTimer(rid string) (*time.Timer, bool) {
	if rmon, ok := m[rid]; ok {
		return rmon.Restart.Timer, true
	} else {
		return nil, false
	}
}

func (m ResourceMonitorMap) HasRestartTimer(rid string) bool {
	if rmon, ok := m[rid]; ok {
		return rmon.Restart.Timer != nil
	} else {
		return false
	}
}

func (m ResourceMonitorMap) SetRestartLastAt(rid string, v time.Time) {
	if rmon, ok := m[rid]; ok {
		rmon.Restart.LastAt = v
		m[rid] = rmon
	}
}

func (m ResourceMonitorMap) SetRestartRemaining(rid string, v int) {
	if rmon, ok := m[rid]; ok {
		rmon.Restart.Remaining = v
		m[rid] = rmon
	}
}

func (m ResourceMonitorMap) SetRestartTimer(rid string, v *time.Timer) {
	if rmon, ok := m[rid]; ok {
		rmon.Restart.Timer = v
		m[rid] = rmon
	}
}
