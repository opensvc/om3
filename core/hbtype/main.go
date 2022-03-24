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
		Kind     string                     `json:"kind"`
		Compat   int                        `json:"compat"`
		Gen      map[string]uint64          `json:"gen"`
		Updated  timestamp.T                `json:"updated"`
		Ping     cluster.NodeMonitor        `json:"monitor"` // monitor from 2.1
		Deltas   map[string]jsondelta.Patch `json:"deltas"`
		Full     cluster.NodeStatus         `json:"full"` // Msg from 2.1
		Nodename string                     `json:"nodename"`
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
