// Package hbtype provides types for hb drivers
package hbtype

import (
	"encoding/json"

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

// New create new Msg from b and nodename
func New(b []byte, nodename string) (*Msg, error) {
	// TODO use full in Msg once 2.1 not anymore needed
	//msg := &Msg{}
	//if err := json.Unmarshal(b, msg); err != nil {
	//	return nil, err
	//}
	msgI := make(map[string]interface{})
	var msg *Msg
	if err := json.Unmarshal(b, &msgI); err != nil {
		return nil, err
	}
	if _, ok := msgI["kind"]; ok {
		msg = &Msg{
			Nodename: nodename,
		}
		for k, v := range msgI {
			tmp, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			switch k {
			case "kind":
				if err := json.Unmarshal(tmp, &msg.Kind); err != nil {
					return nil, err
				}
			case "compat":
				if err := json.Unmarshal(tmp, &msg.Compat); err != nil {
					return nil, err
				}
			case "gen":
				if err := json.Unmarshal(tmp, &msg.Gen); err != nil {
					return nil, err
				}
			case "updated":
				if err := json.Unmarshal(tmp, &msg.Updated); err != nil {
					return nil, err
				}
			case "ping":
				if err := json.Unmarshal(tmp, &msg.Ping); err != nil {
					return nil, err
				}
			case "deltas":
				if err := json.Unmarshal(tmp, &msg.Deltas); err != nil {
					return nil, err
				}
			}
		}
	} else {
		full := &cluster.NodeStatus{}
		if err := json.Unmarshal(b, &full); err != nil {
			return nil, err
		}
		msg = &Msg{
			Kind:     "full",
			Gen:      full.Gen,
			Full:     *full,
			Nodename: nodename,
		}
	}
	return msg, nil
}
