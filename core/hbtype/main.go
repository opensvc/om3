// Package hbtype provides types for hb drivers
package hbtype

import (
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	// Msg struct holds hb message
	Msg struct {
		Kind     string              `json:"kind"`
		Compat   int                 `json:"compat"`
		Gen      map[string]uint64   `json:"gen"`
		Updated  timestamp.T         `json:"updated"`
		Ping     cluster.NodeMonitor `json:"monitor"` // monitor from 2.1
		Deltas   map[string]Patches  `json:"deltas"`
		Full     cluster.NodeStatus  `json:"full"` // Msg from 2.1
		Nodename string              `json:"nodename"`
	}

	// Patches is a slice of jsondelta.Patch
	Patches []jsondelta.Patch
)
