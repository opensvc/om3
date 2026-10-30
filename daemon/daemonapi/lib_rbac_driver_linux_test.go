//go:build linux

package daemonapi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A section root made to run a driver the namespace administrator may not
// choose is root's whole: the keywords only that driver reads are not opened
// by the policy naming none of them, as the rcmd of a kvm, which runs on the
// node, or the command of a task running there. The common keywords, about the
// lifecycle of the object, stay the administrator's.
func TestASectionRunningADriverTheWriterMayNotChooseIsRoots(t *testing.T) {
	knownNodes(t)
	const kvm = `
[DEFAULT]
nodes = *

[container#1]
type = kvm
name = vm1
`
	assert.Error(t, write(t, kvm, kvm+"rcmd = touch /root/x\n"), "a command run on the node")
	assert.Error(t, write(t, kvm, `
[DEFAULT]
nodes = *

[container#1]
type = kvm
name = ../../../tmp/x
`), "the name locating the domain configuration")
	assert.Error(t, write(t, kvm+"rcmd = true\n", kvm), "taking one away")
	assert.NoError(t, write(t, kvm, kvm+"tags = noaction\n"), "a common keyword")
	assert.NoError(t, write(t, kvm, `
[DEFAULT]
nodes = *
comment = hello

[container#1]
type = kvm
name = vm1
`), "another section")

	const task = `
[DEFAULT]
nodes = *

[task#1]
command = /bin/true
`
	assert.Error(t, write(t, task, `
[DEFAULT]
nodes = *

[task#1]
command = /bin/false
`), "a task naming no type runs on the node")
	assert.Error(t, write(t, task+"type = host\n", task+"type = host\nuser = nobody\n"))

	const docker = `
[DEFAULT]
nodes = *

[container#1]
type = docker
image = ghcr.io/opensvc/pause
`
	assert.NoError(t, write(t, docker, `
[DEFAULT]
nodes = *

[container#1]
type = docker
image = ghcr.io/opensvc/other
`), "a container the engine confines is the administrator's")
}

// A namespace administrator may run a guest made of what the namespace holds,
// with a keyword outside of those refused whichever way it is written.
func TestAGuestOfANamespaceAdministrator(t *testing.T) {
	knownNodes(t)
	const guest = `
[DEFAULT]
nodes = *

[volume#1]
size = 4g

[container#1]
type = kvm
vcpus = 2
memory = 1g
disk = volume#1
network = backend3
image = https://cloud-images.ubuntu.com/noble.img
cloud_init = from ./cfg/ci key user-data
`
	assert.NoError(t, write(t, "", guest))
	assert.Error(t, write(t, "", guest+"disk@n1 = /dev/sda\n"), "scoped to a node")
	assert.Error(t, write(t, "", guest+"name = dev2n2\n"), "the domain of another")
	assert.Error(t, write(t, "", guest+"virtinst_args = --disk /dev/sda\n"))
	assert.Error(t, write(t, guest, guest+"network_filter = false\n"), "the filter stays")

	const env = `
[env]
dev = /dev/sda
`
	assert.Error(t, write(t, env, env+guest+"disk = {env.dev}\n"), "through a reference")

	// A keyword docker reads nothing of says something the day the type
	// changes, which is then the write to refuse.
	const docker = `
[DEFAULT]
nodes = *

[container#1]
type = docker
image = alpine
virtinst_args = --disk /dev/sda
`
	assert.NoError(t, write(t, "", docker), "it means nothing yet")
	assert.Error(t, write(t, docker, strings.Replace(docker, "type = docker", "type = kvm", 1)))

	// The node reaches an encapsulated node by name, as root.
	encap := strings.Replace(guest, "nodes = *", "nodes = *\nencapnodes = dev2n2", 1)
	assert.Error(t, write(t, "", encap))
	assert.Error(t, write(t, guest, encap))
	assert.NoError(t, write(t, encap, encap+"comment = x\n"), "root set it up")
}
