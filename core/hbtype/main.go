// Package hbtype provides types for hb drivers
package hbtype

import (
	"time"

	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/node"
)

type (
	// Msg struct holds all kinds of hb message
	Msg struct {
		Kind      string                   `json:"kind"`
		Compat    uint64                   `json:"compat"`
		Gen       node.Gen                 `json:"gen"`
		UpdatedAt time.Time                `json:"updated_at"`
		Ping      node.Monitor             `json:"monitor,omitempty"`
		Events    map[string][]event.Event `json:"events,omitempty"`
		NodeData  node.Node                `json:"node_data,omitempty"`
		Nodename  string                   `json:"nodename"`
	}

	// IDStopper is the interface to stop a hb driver
	IDStopper interface {
		ID() string
		Stop() error
	}

	// Transmitter is the interface that wraps the basic methods for hb driver to send hb messages
	Transmitter interface {
		IDStopper
		Start(cmdC chan<- interface{}, dataC <-chan []byte) error
	}

	// Receiver is the interface that wraps the basic methods for hb driver to receive hb messages
	Receiver interface {
		IDStopper
		Start(cmdC chan<- interface{}, msgC chan<- *Msg) error
	}
)
