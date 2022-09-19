// Package hbtype provides types for hb drivers
package hbtype

import (
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	// Msg struct holds all kinds of hb message
	Msg struct {
		Kind     string                     `json:"kind"`
		Compat   uint64                     `json:"compat"`
		Gen      map[string]uint64          `json:"gen"`
		Updated  time.Time                  `json:"updated"`
		Ping     cluster.NodeMonitor        `json:"monitor"`
		Deltas   map[string]jsondelta.Patch `json:"deltas"`
		Full     cluster.TNode              `json:"full"`
		Nodename string                     `json:"nodename"`
	}

	// MsgFull struct holds kind full hb message
	MsgFull struct {
		Kind     string            `json:"kind,omitempty"`
		Compat   uint64            `json:"compat,omitempty"`
		Gen      map[string]uint64 `json:"gen,omitempty"`
		Updated  time.Time         `json:"updated,omitempty"`
		Full     cluster.TNode     `json:"full,omitempty"`
		Nodename string            `json:"nodename,omitempty"`
	}

	// MsgPatch struct holds kind patch hb message
	MsgPatch struct {
		Kind     string                     `json:"kind,omitempty"`
		Compat   uint64                     `json:"compat,omitempty"`
		Gen      map[string]uint64          `json:"gen,omitempty"`
		Updated  time.Time                  `json:"updated,omitempty"`
		Deltas   map[string]jsondelta.Patch `json:"deltas,omitempty"`
		Nodename string                     `json:"nodename,omitempty"`
	}

	// MsgPing struct holds kind ping hb message
	MsgPing struct {
		Kind     string              `json:"kind,omitempty"`
		Compat   uint64              `json:"compat,omitempty"`
		Gen      map[string]uint64   `json:"gen,omitempty"`
		Updated  time.Time           `json:"updated,omitempty"`
		Ping     cluster.NodeMonitor `json:"monitor,omitempty"` // monitor from 2.1
		Nodename string              `json:"nodename,omitempty"`
	}

	// Transmitter is the interface that wraps the basic methods for hb driver to send hb messages
	Transmitter interface {
		Start(cmdC chan<- interface{}, dataC <-chan []byte) error
		Stop() error
		Id() string
	}

	// Receiver is the interface that wraps the basic methods for hb driver to receive hb messages
	Receiver interface {
		Start(cmdC chan<- interface{}, msgC chan<- *Msg) error
		Stop() error
		Id() string
	}
)
