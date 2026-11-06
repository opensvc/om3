//go:build linux

package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/hostname"
)

// create runs the policy over an object created with the configuration, and
// returns the refusal.
func create(t *testing.T, s string) error {
	t.Helper()
	return configRbacChanges(admin, naming.KindSvc, nil, configOf(t, s))
}

// A keyword a container does not write takes the value of DEFAULT, so a
// value the policy refuses in the container is refused in DEFAULT too, when
// a container inherits it.
func TestAContainerInheritsWhatItWouldBeRefused(t *testing.T) {
	knownNodes(t)
	for _, line := range []string{
		"volume_mounts = /etc:/hostetc:ro",
		"privileged = true",
		"run_args = --pid=host",
		"devices = /dev/sda:/dev/sda",
		"netns = host",
		"volume_mounts@" + hostname.Hostname() + " = /etc:/hostetc:ro",
	} {
		err := create(t, `
nodes = *
`+line+`

[container#1]
type = oci
image = alpine
`)
		require.Error(t, err, line)
		assert.Contains(t, err.Error(), "inherited from DEFAULT.", line)
	}
}

// The keywords a set of identical containers share are written once, in
// DEFAULT, as long as each container could have written them itself.
func TestContainersShareWhatTheyMayWrite(t *testing.T) {
	knownNodes(t)
	assert.NoError(t, create(t, `
nodes = *
image = alpine
rm = true
restart = 3
volume_mounts = foo-data/x:/data
environment = A=1

[container#1]
type = oci

[container#2]
type = oci
`))
}

// A DEFAULT keyword no section inherits changes nothing: it is judged when a
// section comes to inherit it.
func TestADefaultNothingInheritsIsLetIn(t *testing.T) {
	knownNodes(t)
	assert.NoError(t, create(t, `
nodes = *
privileged = true
`))
}

// A section runs the driver it names, never the one DEFAULT names, so a
// section naming none runs the fallback of its group, which is judged.
func TestDefaultTypeChoosesNoDriver(t *testing.T) {
	knownNodes(t)
	assert.Error(t, create(t, `
nodes = *
type = host
command = /bin/true

[task#1]
`), "a task naming no type runs on the node")

	assert.Error(t, create(t, `
nodes = *
type = nfs
path = /
opts = *(rw,no_root_squash)

[share#1]
`), "the share exports what DEFAULT says")
}

// What a write changes includes what a section inherits: a DEFAULT keyword
// written, a section added under one, a keyword of a section taken away,
// which uncovers the DEFAULT one.
func TestAWriteAnswersForWhatSectionsInherit(t *testing.T) {
	knownNodes(t)
	const container = `
nodes = *

[container#1]
type = oci
image = alpine
`
	err := write(t, container, `
nodes = *
privileged = true

[container#1]
type = oci
image = alpine
`)
	require.Error(t, err, "a DEFAULT keyword written")
	assert.Contains(t, err.Error(), "container#1.privileged=true, inherited from DEFAULT.privileged")

	const rootDefault = `
nodes = *
privileged = true

[container#1]
type = oci
image = alpine
privileged = false
`
	assert.Error(t, write(t, rootDefault, rootDefault+`
[container#2]
type = oci
image = alpine
`), "a section added under a value root wrote in DEFAULT")

	assert.NoError(t, write(t, rootDefault, rootDefault+`
[container#2]
type = oci
image = alpine
privileged = false
`), "a section added writing the value it may")

	assert.Error(t, write(t, rootDefault, `
nodes = *
privileged = true

[container#1]
type = oci
image = alpine
`), "a keyword taken away uncovers the DEFAULT one")

	assert.NoError(t, write(t, rootDefault, rootDefault+"\n[env]\na = 1\n"),
		"a write changing nothing a section inherits")
}

// The source an environment reference seeds its key from is judged as the
// section reads it, inherited from DEFAULT or through a reference: a file of
// the node is the server's to read as root, a key generated or fetched reads
// nothing of the node.
func TestAnEnvironmentSourceIsJudgedAsTheSectionReadsIt(t *testing.T) {
	knownNodes(t)
	err := create(t, `
nodes = *
environment = X from ./sec/a key k source /etc/shadow

[container#1]
type = oci
image = alpine
`)
	require.Error(t, err, "inherited from DEFAULT")
	assert.Contains(t, err.Error(), "inherited from DEFAULT.environment")
	assert.Contains(t, err.Error(), "a server-local source uri requires the root grant")

	assert.NoError(t, create(t, `
nodes = *
environment = X from ./sec/a key k source random

[container#1]
type = oci
image = alpine
`), "a key generated, inherited from DEFAULT")

	err = create(t, `
nodes = *

[env]
x = X from ./sec/a key k source /etc/shadow

[container#1]
type = oci
image = alpine
environment = {env.x}
`)
	require.Error(t, err, "read through a reference")
	assert.Contains(t, err.Error(), "a server-local source uri requires the root grant")

	assert.NoError(t, create(t, `
nodes = *

[env]
x = X from ./sec/a key k source random

[container#1]
type = oci
image = alpine
environment = {env.x}
`), "a key generated, read through a reference")

	err = create(t, `
nodes = *

[container#1]
type = oci
image = alpine
environment = A=1 X from ./sec/a key k source /etc/shadow
`)
	require.Error(t, err, "written in the section")
	assert.Contains(t, err.Error(), "a server-local source uri requires the root grant")
}
