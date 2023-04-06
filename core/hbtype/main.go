// Package hbtype provides types for hb drivers
package hbtype

import (
	"time"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/node"
)

type (
	// Msg struct holds all kinds of hb message
	Msg struct {
		Kind     string                     `json:"kind"`
		Compat   uint64                     `json:"compat"`
		Gen      map[string]uint64          `json:"gen"`
		Updated  time.Time                  `json:"updated"`
		Ping     node.Monitor               `json:"monitor"`
		Events   map[string][]event.Event   `json:"events,omitempty"`
		Full     node.Node                  `json:"full"`
		Nodename string                     `json:"nodename"`
	}

	// MsgFull struct holds kind full hb message
	MsgFull struct {
		Kind     string            `json:"kind,omitempty"`
		Compat   uint64            `json:"compat,omitempty"`
		Gen      map[string]uint64 `json:"gen,omitempty"`
		Updated  time.Time         `json:"updated,omitempty"`
		Full     node.Node         `json:"full,omitempty"`
		Nodename string            `json:"nodename,omitempty"`
	}

	// MsgPatch struct holds kind patch hb message
	MsgPatch struct {
		Kind     string                     `json:"kind,omitempty"`
		Compat   uint64                     `json:"compat,omitempty"`
		Gen      map[string]uint64          `json:"gen,omitempty"`
		Updated  time.Time                  `json:"updated,omitempty"`
		Events   map[string][]event.Event   `json:"events,omitempty"`
		Nodename string                     `json:"nodename,omitempty"`
	}

	// MsgPing struct holds kind ping hb message
	MsgPing struct {
		Kind     string            `json:"kind,omitempty"`
		Compat   uint64            `json:"compat,omitempty"`
		Gen      map[string]uint64 `json:"gen,omitempty"`
		Updated  time.Time         `json:"updated,omitempty"`
		Ping     node.Monitor      `json:"monitor,omitempty"` // monitor from 2.1
		Nodename string            `json:"nodename,omitempty"`
	}

	// IdStopper
	IdStopper interface {
		Id() string
		Stop() error
	}

	// Transmitter is the interface that wraps the basic methods for hb driver to send hb messages
	Transmitter interface {
		IdStopper
		Start(cmdC chan<- interface{}, dataC <-chan []byte) error
	}

	// Receiver is the interface that wraps the basic methods for hb driver to receive hb messages
	Receiver interface {
		IdStopper
		Start(cmdC chan<- interface{}, msgC chan<- *Msg) error
	}
)
