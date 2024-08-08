package instance

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/xmap"
)

type (
	// Monitor describes the in-daemon states of an instance
	Monitor struct {
		GlobalExpect          MonitorGlobalExpect `json:"global_expect"`
		GlobalExpectUpdatedAt time.Time           `json:"global_expect_updated_at"`
		GlobalExpectOptions   any                 `json:"global_expect_options"`

		// IsLeader flags the instance as the one where to provision as leader.
		// The provisioning leader is responsible for preparing the shared resources.
		// There can be only one leader, whatever the topology.
		IsLeader bool `json:"is_leader"`

		// IsHALeader flags the instances to start automatically if orchestrate=ha
		// or when the admin posted a start orchestration.
		// There can be one leader on a failover object, or many leaders with a flex topology.
		IsHALeader bool `json:"is_ha_leader"`

		LocalExpect          MonitorLocalExpect `json:"local_expect"`
		LocalExpectUpdatedAt time.Time          `json:"local_expect_updated_at"`

		// OrchestrationID is the accepted orchestration id that will be unset
		// when orchestration is reached on local node
		OrchestrationID uuid.UUID `json:"orchestration_id"`

		// OrchestrationIsDone is set by the orchestration when it decides the instance state has reached its target.
		// A orchestration is cleaned up when all instance monitors have OrchestrationIsDone set.
		OrchestrationIsDone bool `json:"orchestration_is_done"`

		SessionID               uuid.UUID        `json:"session_id"`
		State                   MonitorState     `json:"state"`
		StateUpdatedAt          time.Time        `json:"state_updated_at"`
		MonitorActionExecutedAt time.Time        `json:"monitor_action_executed_at"`
		IsPreserved             bool             `json:"preserved"`
		Resources               ResourceMonitors `json:"resources,omitempty"`
		UpdatedAt               time.Time        `json:"updated_at"`

		Parents  map[string]status.T `json:"parents,omitempty"`
		Children map[string]status.T `json:"children,omitempty"`
	}

	ResourceMonitors map[string]ResourceMonitor

	// MonitorUpdate is embedded in the SetInstanceMonitor message to
	// change some Monitor values. A nil value does not change the
	// current value.
	MonitorUpdate struct {
		GlobalExpect        *MonitorGlobalExpect `json:"global_expect"`
		GlobalExpectOptions any                  `json:"global_expect_options"`
		LocalExpect         *MonitorLocalExpect  `json:"local_expect"`
		State               *MonitorState        `json:"state"`

		// CandidateOrchestrationID is a candidate orchestration id for a new imon orchestration.
		CandidateOrchestrationID uuid.UUID `json:"orchestration_id"`
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
	MonitorStateInit MonitorState = iota
	MonitorStateBooted
	MonitorStateBootFailed
	MonitorStateBooting
	MonitorStateIdle
	MonitorStateInterrupted
	MonitorStateDeleted
	MonitorStateDeleteFailed
	MonitorStateDeleting
	MonitorStateFreezeFailed
	MonitorStateFreezing
	MonitorStateFrozen
	MonitorStateProvisioned
	MonitorStateProvisioning
	MonitorStateProvisionFailed
	MonitorStatePurgeFailed
	MonitorStateReady
	MonitorStateRestarted
	MonitorStateRunning
	MonitorStateShutdownFailed
	MonitorStateShutdown
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
	MonitorStateWaitChildren
	MonitorStateWaitLeader
	MonitorStateWaitNonLeader
	MonitorStateWaitParents
	MonitorStateWaitPriors
)

const (
	MonitorLocalExpectInit MonitorLocalExpect = iota
	MonitorLocalExpectNone
	MonitorLocalExpectStarted
	MonitorLocalExpectShutdown
)

const (
	MonitorGlobalExpectInit MonitorGlobalExpect = iota
	MonitorGlobalExpectAborted
	MonitorGlobalExpectDeleted
	MonitorGlobalExpectFrozen
	MonitorGlobalExpectNone
	MonitorGlobalExpectPlaced
	MonitorGlobalExpectPlacedAt
	MonitorGlobalExpectProvisioned
	MonitorGlobalExpectPurged
	MonitorGlobalExpectRestarted
	MonitorGlobalExpectStarted
	MonitorGlobalExpectStopped
	MonitorGlobalExpectThawed
	MonitorGlobalExpectUnprovisioned
)

var (
	MonitorStateStrings = map[MonitorState]string{
		MonitorStateBooted:            "booted",
		MonitorStateBootFailed:        "boot failed",
		MonitorStateBooting:           "booting",
		MonitorStateDeleted:           "deleted",
		MonitorStateDeleteFailed:      "delete failed",
		MonitorStateDeleting:          "deleting",
		MonitorStateFreezeFailed:      "freeze failed",
		MonitorStateFreezing:          "freezing",
		MonitorStateFrozen:            "frozen",
		MonitorStateIdle:              "idle",
		MonitorStateInterrupted:       "interrupted",
		MonitorStateProvisioned:       "provisioned",
		MonitorStateProvisioning:      "provisioning",
		MonitorStateProvisionFailed:   "provision failed",
		MonitorStatePurgeFailed:       "purge failed",
		MonitorStateReady:             "ready",
		MonitorStateRestarted:         "restarted",
		MonitorStateRunning:           "running",
		MonitorStateShutdown:          "shutdown",
		MonitorStateShutdownFailed:    "shutdown failed",
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
		MonitorStateWaitChildren:      "wait children",
		MonitorStateWaitLeader:        "wait leader",
		MonitorStateWaitNonLeader:     "wait non-leader",
		MonitorStateWaitParents:       "wait parents",
		MonitorStateWaitPriors:        "wait priors",
		MonitorStateInit:              "init",
	}

	MonitorStateValues = map[string]MonitorState{
		"init":               MonitorStateInit,
		"booted":             MonitorStateBooted,
		"boot failed":        MonitorStateBootFailed,
		"booting":            MonitorStateBooting,
		"idle":               MonitorStateIdle,
		"interrupted":        MonitorStateInterrupted,
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
		"restarted":          MonitorStateRestarted,
		"running":            MonitorStateRunning,
		"shutdown":           MonitorStateShutdown,
		"shutdown failed":    MonitorStateShutdownFailed,
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
		"wait children":      MonitorStateWaitChildren,
		"wait leader":        MonitorStateWaitLeader,
		"wait non-leader":    MonitorStateWaitNonLeader,
		"wait parents":       MonitorStateWaitParents,
		"wait priors":        MonitorStateWaitPriors,
	}

	MonitorLocalExpectStrings = map[MonitorLocalExpect]string{
		MonitorLocalExpectStarted:  "started",
		MonitorLocalExpectShutdown: "shutdown",
		MonitorLocalExpectNone:     "none",
		MonitorLocalExpectInit:     "init",
	}

	MonitorLocalExpectValues = map[string]MonitorLocalExpect{
		"shutdown": MonitorLocalExpectShutdown,
		"started":  MonitorLocalExpectStarted,
		"none":     MonitorLocalExpectNone,
		"init":     MonitorLocalExpectInit,
	}

	MonitorGlobalExpectStrings = map[MonitorGlobalExpect]string{
		MonitorGlobalExpectAborted:       "aborted",
		MonitorGlobalExpectDeleted:       "deleted",
		MonitorGlobalExpectInit:          "init",
		MonitorGlobalExpectFrozen:        "frozen",
		MonitorGlobalExpectNone:          "none",
		MonitorGlobalExpectPlaced:        "placed",
		MonitorGlobalExpectPlacedAt:      "placed@",
		MonitorGlobalExpectProvisioned:   "provisioned",
		MonitorGlobalExpectPurged:        "purged",
		MonitorGlobalExpectRestarted:     "restarted",
		MonitorGlobalExpectStarted:       "started",
		MonitorGlobalExpectStopped:       "stopped",
		MonitorGlobalExpectThawed:        "thawed",
		MonitorGlobalExpectUnprovisioned: "unprovisioned",
	}

	MonitorGlobalExpectValues = map[string]MonitorGlobalExpect{
		"aborted":       MonitorGlobalExpectAborted,
		"deleted":       MonitorGlobalExpectDeleted,
		"init":          MonitorGlobalExpectInit,
		"frozen":        MonitorGlobalExpectFrozen,
		"placed":        MonitorGlobalExpectPlaced,
		"placed@":       MonitorGlobalExpectPlacedAt,
		"provisioned":   MonitorGlobalExpectProvisioned,
		"purged":        MonitorGlobalExpectPurged,
		"restarted":     MonitorGlobalExpectRestarted,
		"started":       MonitorGlobalExpectStarted,
		"stopped":       MonitorGlobalExpectStopped,
		"thawed":        MonitorGlobalExpectThawed,
		"unprovisioned": MonitorGlobalExpectUnprovisioned,
		"none":          MonitorGlobalExpectNone,
	}

	ErrInvalidGlobalExpect = errors.New("invalid instance monitor global expect")
	ErrInvalidLocalExpect  = errors.New("invalid instance monitor local expect")
	ErrInvalidState        = errors.New("invalid instance monitor state")
	ErrSameGlobalExpect    = errors.New("instance monitor global expect is already set to the same value")
	ErrSameLocalExpect     = errors.New("instance monitor local expect is already set to the same value")
	ErrSameState           = errors.New("instance monitor state is already set to the same value")

	MonitorActionNone       MonitorAction = "none"
	MonitorActionCrash      MonitorAction = "crash"
	MonitorActionFreezeStop MonitorAction = "freeze_stop"
	MonitorActionReboot     MonitorAction = "reboot"
	MonitorActionSwitch     MonitorAction = "switch"
)

func (t MonitorState) Is(states ...MonitorState) bool {
	for _, s := range states {
		if s == t {
			return true
		}
	}
	return false
}

func (t MonitorState) IsDoing() bool {
	return strings.HasSuffix(t.String(), "ing")
}

func (t MonitorState) String() string {
	return MonitorStateStrings[t]
}

func (t MonitorState) MarshalText() ([]byte, error) {
	if s, ok := MonitorStateStrings[t]; !ok {
		return []byte{}, fmt.Errorf("unexpected MonitorState value: %d", t)
	} else {
		return []byte(s), nil
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

func (t *MonitorState) UnmarshalText(b []byte) error {
	s := string(b)
	if v, ok := MonitorStateValues[s]; !ok {
		return fmt.Errorf("unexpected MonitorState value: %s", s)
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
		return []byte{}, fmt.Errorf("unexpected MonitorLocalExpect value: %d", t)
	} else {
		return []byte(s), nil
	}
}

func (t *MonitorLocalExpect) UnmarshalText(b []byte) error {
	s := string(b)
	if v, ok := MonitorLocalExpectValues[s]; !ok {
		return fmt.Errorf("unexpected MonitorLocalExpect value: %s", s)
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
		return []byte{}, fmt.Errorf("unexpected MonitorGlobalExpect value: %d", t)
	} else {
		return []byte(s), nil
	}
}

func (t *MonitorGlobalExpect) UnmarshalText(b []byte) error {
	s := string(b)
	if v, ok := MonitorGlobalExpectValues[s]; !ok {
		return fmt.Errorf("unexpected MonitorGlobalExpect value: %s", s)
	} else {
		*t = v
		return nil
	}
}

func (rmon *ResourceMonitor) DecRestartRemaining() {
	if rmon.Restart.Remaining > 0 {
		rmon.Restart.Remaining -= 1
	}
}

func (rmon *ResourceMonitor) StopRestartTimer() bool {
	if rmon.Restart.Timer == nil {
		return false
	} else {
		rmon.Restart.Timer.Stop()
		rmon.Restart.Timer = nil
		return true
	}
}

func (m ResourceMonitors) Set(rid string, rmon ResourceMonitor) {
	m[rid] = rmon
}

func (m ResourceMonitors) Get(rid string) *ResourceMonitor {
	if rmon, ok := m[rid]; ok {
		return &rmon
	} else {
		return nil
	}
}

func (m ResourceMonitors) DeepCopy() ResourceMonitors {
	return xmap.Copy(m)
}

func (mon Monitor) ResourceFlagRestartString(rid resourceid.T, r resource.Status) string {
	// Restart and retries
	retries := 0
	if rmon := mon.Resources.Get(rid.Name); rmon != nil {
		retries = rmon.Restart.Remaining
	}
	return r.Restart.FlagString(retries)
}

func (mon Monitor) DeepCopy() *Monitor {
	v := mon
	v.Resources = v.Resources.DeepCopy()
	if mon.GlobalExpectOptions != nil {
		switch mon.GlobalExpect {
		case MonitorGlobalExpectPlacedAt:
			b, _ := json.Marshal(mon.GlobalExpectOptions)
			var placedAt MonitorGlobalExpectOptionsPlacedAt
			// TODO Don't ignore following error
			_ = json.Unmarshal(b, &placedAt)
			v.GlobalExpectOptions = placedAt
		// TODO add other cases for globalExpect values that requires GlobalExpectOptions
		default:
			b, _ := json.Marshal(mon.GlobalExpectOptions)
			// TODO Don't ignore following error
			_ = json.Unmarshal(b, &v.GlobalExpectOptions)
		}
	}
	return &v
}

func (t ResourceMonitor) Unstructured() map[string]any {
	return map[string]any{
		"restart": t.Restart.Unstructured(),
	}
}

func (t ResourceMonitorRestart) Unstructured() map[string]any {
	return map[string]any{
		"remaining": t.Remaining,
		"last_at":   t.LastAt,
	}
}

func (t Monitor) Unstructured() map[string]any {
	m := map[string]any{
		"global_expect":              t.GlobalExpect,
		"global_expect_updated_at":   t.GlobalExpectUpdatedAt,
		"global_expect_options":      t.GlobalExpectOptions,
		"is_leader":                  t.IsLeader,
		"is_ha_leader":               t.IsHALeader,
		"local_expect":               t.LocalExpect,
		"local_expect_updated_at":    t.LocalExpectUpdatedAt,
		"orchestration_id":           t.OrchestrationID,
		"orchestration_is_done":      t.OrchestrationIsDone,
		"session_id":                 t.SessionID,
		"state":                      t.State,
		"state_updated_at":           t.StateUpdatedAt,
		"monitor_action_executed_at": t.MonitorActionExecutedAt,
		"preserved":                  t.IsPreserved,
		"updated_at":                 t.UpdatedAt,
	}
	if len(t.Resources) > 0 {
		m["resources"] = t.Resources.Unstructured()
	}
	if len(t.Parents) > 0 {
		m["parents"] = t.Parents
	}
	if len(t.Children) > 0 {
		m["children"] = t.Children
	}
	return m
}

func (t ResourceMonitors) Unstructured() map[string]map[string]any {
	m := make(map[string]map[string]any)
	for k, v := range t {
		m[k] = v.Unstructured()
	}
	return m
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
	if t.GlobalExpectOptions != nil {
		s += fmt.Sprintf(" GlobalExpectOptions=%#v", t.GlobalExpectOptions)
	}
	return fmt.Sprintf("instance.MonitorUpdate{%s}", s)
}
