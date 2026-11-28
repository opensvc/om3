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
		ID     string `json:"ID"`
	}

	// dockerEvent is the part of a "docker events --format '{{json .}}'"
	// line the monitor reads.
	dockerEvent struct {
		Type   string `json:"Type"`
		Action string `json:"Action"`
		Actor  struct {
			ID         string            `json:"ID"`
			Attributes map[string]string `json:"Attributes"`
		} `json:"Actor"`
	}

	// imageRemoval is an image event saying an image was untagged or
	// removed from the store of an engine: its name when the engine
	// tells one, and its id.
	imageRemoval struct {
		name string
		id   string
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

// imageRemovalActions are the image events after which the store of an
// engine may not hold an image it held: docker says untag and delete,
// podman untag and remove. The prune docker says after its deletes names no
// image, and adds nothing to them.
var imageRemovalActions = map[string]bool{
	"untag":  true,
	"delete": true,
	"remove": true,
}

// newImageRemoval returns the removal of the image an event names, with the
// name left out when it names no image: the engines also put the id there,
// whole, with its algorithm, or cut short.
func newImageRemoval(name, id string) imageRemoval {
	bare := strings.TrimPrefix(id, "sha256:")
	if strings.HasPrefix(name, "sha256:") || (name != "" && strings.HasPrefix(bare, name)) {
		name = ""
	}
	return imageRemoval{name: name, id: id}
}

// parsePodmanImageEvent returns the image a podman event line says was
// untagged or removed. The lines podman 4.9.3 prints for an rmi of a tag:
//
//	{"ID":"e2ac…","Name":"docker.io/library/hello-world:linux","Status":"untag","Time":"…","Type":"image","Attributes":null}
//	{"ID":"e2ac…","Name":"docker.io/library/hello-world:linux","Status":"remove","Time":"…","Type":"image","Attributes":null}
//
// The remove of a prune has no Name, and the one of an rmi by id has the id
// cut short as its Name.
func parsePodmanImageEvent(line string) (imageRemoval, bool) {
	var ev podmanEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return imageRemoval{}, false
	}
	if ev.Type != "image" || !imageRemovalActions[ev.Status] || (ev.Name == "" && ev.ID == "") {
		return imageRemoval{}, false
	}
	return newImageRemoval(ev.Name, ev.ID), true
}

// parseDockerImageEvent returns the image a docker event line says was
// untagged or deleted. The lines docker 29.1.3 prints for an rmi of a tag,
// as for a prune, name the image by its id only:
//
//	{"Type":"image","Action":"untag","Actor":{"ID":"sha256:e2ac…","Attributes":{"name":"sha256:e2ac…"}},…}
//	{"Type":"image","Action":"delete","Actor":{"ID":"sha256:e2ac…","Attributes":{"name":"sha256:e2ac…"}},…}
func parseDockerImageEvent(line string) (imageRemoval, bool) {
	var ev dockerEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return imageRemoval{}, false
	}
	name := ev.Actor.Attributes["name"]
	if ev.Type != "image" || !imageRemovalActions[ev.Action] || (name == "" && ev.Actor.ID == "") {
		return imageRemoval{}, false
	}
	return newImageRemoval(name, ev.Actor.ID), true
}
