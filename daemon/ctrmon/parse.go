package ctrmon

import (
	"encoding/json"
	"strings"
)

type (
	// podmanEvent is the part of a "podman events --format json" line the
	// monitor reads.
	podmanEvent struct {
		Type   string `json:"Type"`
		Name   string `json:"Name"`
		Status string `json:"Status"`
	}

	// dockerEvent is the part of a "docker events --format '{{json .}}'"
	// line the monitor reads.
	dockerEvent struct {
		Type   string `json:"Type"`
		Action string `json:"Action"`
		Actor  struct {
			Attributes map[string]string `json:"Attributes"`
		} `json:"Actor"`
	}
)

// stateActions are the container events changing what the status of a
// container reads: running, paused, gone, healthy. The others, as the
// create, init, attach and cleanup of a start, or the exec_* events a health
// check or an "om exec" into the container print many of, change nothing of
// it, and refresh nothing.
var stateActions = map[string]bool{
	"start":   true,
	"restart": true,
	"stop":    true,
	"die":     true,
	"died":    true,
	"oom":     true,
	"pause":   true,
	"unpause": true,
	"remove":  true,
	"destroy": true,
	"rename":  true,
}

// isStateAction says the action of a container event changes its status. A
// health status is one, its action holding the status after a colon on
// docker, as "health_status: healthy".
func isStateAction(action string) bool {
	if stateActions[action] {
		return true
	}
	return strings.HasPrefix(action, "health_status")
}

// parsePodmanEvent returns the name of the container a podman event line
// says changed state.
//
//	{"ID":"c3be…","Image":"docker.io/library/alpine:3","Name":"omtest","Status":"died","Time":"…","Type":"container"}
func parsePodmanEvent(line string) (string, bool) {
	var ev podmanEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return "", false
	}
	if ev.Type != "container" || ev.Name == "" || !isStateAction(ev.Status) {
		return "", false
	}
	return ev.Name, true
}

// parseDockerEvent returns the name of the container a docker event line
// says changed state.
//
//	{"Type":"container","Action":"die","Actor":{"ID":"56bc…","Attributes":{"name":"omtest",…}},…}
func parseDockerEvent(line string) (string, bool) {
	var ev dockerEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return "", false
	}
	name := ev.Actor.Attributes["name"]
	if ev.Type != "container" || name == "" || !isStateAction(ev.Action) {
		return "", false
	}
	return name, true
}
