package node

import (
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
)

type (
	// Node holds a node DataSet.
	Node struct {
		Instance map[string]instance.Instance `json:"instance"`
		Pool     map[string]pool.Status       `json:"pool"`
		Monitor  Monitor                      `json:"monitor"`
		Stats    Stats                        `json:"stats"`
		Status   Status                       `json:"status"`
		Os       Os                           `json:"os"`
		Config   Config                       `json:"config"`

		Daemon daemonsubsystem.Daemon `json:"daemon"`

		//Locks map[string]Lock `json:"locks"`
	}
)

// DeepCopy returns a copy of the node dataset sharing nothing with it.
//
// A nil Instance or Pool map is kept nil rather than made empty: neither
// field has omitempty, so the two do not serialize alike.
func (n *Node) DeepCopy() *Node {
	if n == nil {
		return nil
	}
	c := *n
	if n.Instance != nil {
		c.Instance = make(map[string]instance.Instance, len(n.Instance))
		for path, instanceData := range n.Instance {
			c.Instance[path] = *instanceData.DeepCopy()
		}
	}
	if n.Pool != nil {
		c.Pool = make(map[string]pool.Status, len(n.Pool))
		for name, poolData := range n.Pool {
			c.Pool[name] = *poolData.DeepCopy()
		}
	}
	c.Monitor = *n.Monitor.DeepCopy()
	c.Stats = *n.Stats.DeepCopy()
	c.Status = *n.Status.DeepCopy()
	c.Os = n.Os.DeepCopy()
	c.Config = *n.Config.DeepCopy()
	c.Daemon = *n.Daemon.DeepCopy()
	return &c
}
