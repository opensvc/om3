//go:build linux

package daemonapi

import (
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
