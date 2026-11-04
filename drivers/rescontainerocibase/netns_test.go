package rescontainerocibase

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A process in the network namespace of the node, as a container run with
// --net host, has no namespace of its own to plug an address in.
func TestTheNetNSOfAProcessInTheNodeNetNSIsNotPrivate(t *testing.T) {
	assert.Equal(t, "", privateNetNSPathOf(os.Getpid()))
}

// A process gone has no namespace.
func TestTheNetNSOfAProcessGoneIsNone(t *testing.T) {
	assert.Equal(t, "", privateNetNSPathOf(1<<30))
}

// inspectExecuter is an executer whose inspect answers i.
type inspectExecuter struct {
	Executer
	i *InspectData
}

func (e inspectExecuter) Inspect(context.Context) (Inspecter, error) {
	return e.i, nil
}

// A container not running has no namespace, whatever SandboxKey of its last
// run the engine still reports: the ip resources in it stop as down.
func TestAContainerNotRunningHasNoNetNS(t *testing.T) {
	stopped := &InspectData{}
	stopped.State.Running = false
	stopped.NetworkSettings.SandboxKey = "/var/run/docker/netns/0123456789ab"
	bt := &BT{executer: inspectExecuter{i: stopped}}
	path, err := bt.NetNSPath(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "", path)

	running := &InspectData{}
	running.State.Running = true
	running.State.Pid = os.Getpid()
	running.NetworkSettings.SandboxKey = "/var/run/docker/netns/0123456789ab"
	bt = &BT{executer: inspectExecuter{i: running}}
	path, err = bt.NetNSPath(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "/var/run/docker/netns/0123456789ab", path)
}
