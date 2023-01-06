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
		GlobalExpect        MonitorGlobalExpect       `json:"global_expect"`
		GlobalExpectUpdated time.Time                 `json:"global_expect_updated"`
		GlobalExpectOptions any                       `json:"global_expect_options"`
		IsLeader            bool                      `json:"is_leader"`
		IsHALeader          bool                      `json:"is_ha_leader"`
		LocalExpect         MonitorLocalExpect        `json:"local_expect"`
		LocalExpectUpdated  time.Time                 `json:"local_expect_updated"`
		SessionId           string                    `json:"session_id"`
		State               MonitorState              `json:"state"`
		StateUpdated        time.Time                 `json:"state_updated"`
		Restart             map[string]MonitorRestart `json:"restart,omitempty"`
	}

	// MonitorUpdate is embedded in the SetInstanceMonitor message to
	// change some Monitor values. A nil value does not change the
	// current value.
	MonitorUpdate struct {
		GlobalExpect        *MonitorGlobalExpect `json:"global_expect"`
		GlobalExpectOptions any                  `json:"global_expect_options"`
		LocalExpect         *MonitorLocalExpect  `json:"local_expect"`
		State               *MonitorState        `json:"state"`
	}

	// MonitorRestart describes the restart states maintained by the daemon
	// for an object instance.
	MonitorRestart struct {
		Retries int       `json:"retries"`
		Updated time.Time `json:"updated"`
	}

	MonitorState        int
	MonitorLocalExpect  int
	MonitorGlobalExpect int

	MonitorGlobalExpectOptionsPlacedAt struct {
		Destination []string `json:"destination"`
	}
)

const (
	MonitorStateEmpty MonitorState = iota
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
	MonitorLocalExpectUnset MonitorLocalExpect = iota
	MonitorLocalExpectStarted
)

const (
	MonitorGlobalExpectEmpty MonitorGlobalExpect = iota
	MonitorGlobalExpectAborted
	MonitorGlobalExpectFrozen
	MonitorGlobalExpectPlaced
	MonitorGlobalExpectPlacedAt
	MonitorGlobalExpectProvisioned
	MonitorGlobalExpectPurged
	MonitorGlobalExpectStarted
	MonitorGlobalExpectStopped
	MonitorGlobalExpectThawed
	MonitorGlobalExpectUnprovisioned
	MonitorGlobalExpectUnset
)

var (
	MonitorStateStrings = map[MonitorState]string{
		MonitorStateDeleted:           "deleted",
		MonitorStateDeleting:          "deleting",
		MonitorStateEmpty:             "",
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
	}

	MonitorStateValues = map[string]MonitorState{
		"":                   MonitorStateEmpty,
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
		MonitorLocalExpectUnset:   "unset",
	}

	MonitorLocalExpectValues = map[string]MonitorLocalExpect{
		"started": MonitorLocalExpectStarted,
		"unset":   MonitorLocalExpectUnset,
	}

	MonitorGlobalExpectStrings = map[MonitorGlobalExpect]string{
		MonitorGlobalExpectAborted:       "aborted",
		MonitorGlobalExpectEmpty:         "",
		MonitorGlobalExpectFrozen:        "frozen",
		MonitorGlobalExpectPlaced:        "placed",
		MonitorGlobalExpectPlacedAt:      "placed@",
		MonitorGlobalExpectProvisioned:   "provisioned",
		MonitorGlobalExpectPurged:        "purged",
		MonitorGlobalExpectStarted:       "started",
		MonitorGlobalExpectStopped:       "stopped",
		MonitorGlobalExpectThawed:        "thawed",
		MonitorGlobalExpectUnprovisioned: "unprovisioned",
		MonitorGlobalExpectUnset:         "unset",
	}

	MonitorGlobalExpectValues = map[string]MonitorGlobalExpect{
		"aborted":       MonitorGlobalExpectAborted,
		"":              MonitorGlobalExpectEmpty,
		"frozen":        MonitorGlobalExpectFrozen,
		"placed":        MonitorGlobalExpectPlaced,
		"placed@":       MonitorGlobalExpectPlacedAt,
		"provisioned":   MonitorGlobalExpectProvisioned,
		"purged":        MonitorGlobalExpectPurged,
		"started":       MonitorGlobalExpectStarted,
		"stopped":       MonitorGlobalExpectStopped,
		"thawed":        MonitorGlobalExpectThawed,
		"unprovisioned": MonitorGlobalExpectUnprovisioned,
		"unset":         MonitorGlobalExpectUnset,
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
