//go:build linux

package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/hostname"

	// The policy is about driver keywords, which exist only once the drivers
	// are registered. Without this the configurations below evaluate to
	// nothing and the tests pass for the wrong reason.
	_ "github.com/opensvc/om3/v3/core/driverdb"
)

// admin is a user holding the object administrator role on the namespace, and
// nothing else. It is who the policy is written against: root is let through
// before the policy is asked.
var admin = rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "test")}

func configOf(t *testing.T, s string) *xconfig.T {
	t.Helper()
	p, err := naming.ParsePath("test/svc/foo")
	require.NoError(t, err)
	o, err := object.New(p, object.WithConfigData([]byte(s)), object.WithVolatile(true))
	require.NoError(t, err)
	return o.(object.Configurer).Config()
}

// write runs the policy over what a write changes, and returns the refusal.
func write(t *testing.T, from, to string) error {
	t.Helper()
	p, _ := naming.ParsePath("test/svc/foo")
	return configRbacChanges(admin, p.Kind, configOf(t, from), configOf(t, to))
}

// A configuration a user may not have written is not one they may not change.
// The keywords already there were let in by whoever wrote them.
func TestAWriteAnswersForWhatItChanges(t *testing.T) {
	const root = `
nodes = *

[fs#1]
type = xfs
dev = /dev/foo
mnt = /srv/foo
`
	assert.Error(t, configRbacChanges(admin, naming.KindSvc, nil, configOf(t, root)),
		"creating it answers for every keyword of it")

	assert.NoError(t, write(t, root, root+"\ncomment = hello\n"),
		"a comment on an object that mounts a filesystem")

	assert.Error(t, write(t, root, `
nodes = *

[fs#1]
type = xfs
dev = /dev/evil
mnt = /srv/foo
`), "the device the filesystem mounts is the node's to decide")
}

// A keyword taken away is a change like one made. A section deleted is every
// keyword of it unset.
func TestAWriteAnswersForWhatItTakesAway(t *testing.T) {
	const from = `
nodes = *

[fs#1]
type = xfs
dev = /dev/foo
mnt = /srv/foo

[env]
a = 1
`
	err := write(t, from, `
nodes = *

[env]
a = 1
`)
	require.Error(t, err, "the whole filesystem section is deleted")
	assert.Contains(t, err.Error(), "unset")

	assert.Error(t, write(t, from, `
nodes = *

[fs#1]
dev = /dev/foo
mnt = /srv/foo

[env]
a = 1
`), "the filesystem type alone is unset")

	assert.NoError(t, write(t, from, `
nodes = *

[fs#1]
type = xfs
dev = /dev/foo
mnt = /srv/foo
`), "an env keyword is nobody's to be refused")
}

// A keyword can hold a reference, so moving what it refers to rewrites it
// without touching it. References are followed however deep they go.
func TestAWriteAnswersForTheReferencesItMoves(t *testing.T) {
	const from = `
nodes = *

[env]
deep = /bin/true
cmd = {env.deep}

[app#1]
type = forking
start = /bin/true
pre_start = {env.cmd}
`
	// env is a section of values the object reads, and no keyword of it needs
	// a grant. This one is read by a trigger.
	assert.Error(t, write(t, from, `
nodes = *

[env]
deep = /bin/true
cmd = /bin/evil

[app#1]
type = forking
start = /bin/true
pre_start = {env.cmd}
`), "the trigger now runs something else, though nobody wrote the trigger")

	assert.Error(t, write(t, from, `
nodes = *

[env]
deep = /bin/evil
cmd = {env.deep}

[app#1]
type = forking
start = /bin/true
pre_start = {env.cmd}
`), "two references deep is the same trigger running something else")

	assert.NoError(t, write(t, from, from+"\n[env]\nunrelated = 1\n"),
		"an env keyword no trigger reads")
}

// A keyword is written once per node, so a reference that resolves the same
// here can resolve to something else on a peer.
func TestAWriteAnswersForAReferenceItMovesOnAPeer(t *testing.T) {
	// A node the cluster does not hold is a node the object does not run on,
	// and a keyword scoped to it is inert, so the peer has to be a real one.
	scopes := rbacScopes(nil, configOf(t, "nodes = *\n"))
	if len(scopes) < 2 {
		t.Skip("needs a cluster with a peer")
	}
	peer := scopes[0]
	if peer == hostname.Hostname() {
		peer = scopes[1]
	}
	// The keyword resolves here whatever the peer holds, so what the write
	// moves is only what the trigger runs there. Nothing this node reads
	// changes.
	config := func(cmd string) string {
		return `
nodes = *

[env]
cmd = /bin/true
cmd@` + peer + ` = ` + cmd + `

[app#1]
type = forking
start = /bin/true
pre_start = {env.cmd}
`
	}
	assert.NoError(t, write(t, config("/bin/true"), config("/bin/true")))
	assert.Error(t, write(t, config("/bin/true"), config("/bin/evil")),
		"the trigger runs something else on the peer, and nothing changed here")
}

// A section that writes no type runs the driver its group falls back to. The
// policy is asked about keywords, and there is no keyword here to ask about,
// so the fallback is checked as though it had been written.
func TestAWriteAnswersForTheDriverASectionDoesNotName(t *testing.T) {
	create := func(config string) error {
		return configRbacChanges(admin, naming.KindSvc, nil, configOf(t, config))
	}

	// A task falls back to the host driver, which runs its command on the
	// node. Writing that type is refused, so leaving it out cannot be the way
	// to get it.
	assert.Error(t, create(`
nodes = *

[task#1]
command = /bin/evil
schedule = @1
`), "the task runs its command on the node, and never says so")

	assert.Error(t, create(`
nodes = *

[ip#1]
network = default
`), "an ip falls back to the host driver too")

	// A container falls back to oci, which is a type the policy opens, so
	// leaving it out is as writable as writing it.
	assert.NoError(t, create(`
nodes = *

[container#0]
image = busybox
`))

	assert.NoError(t, create(`
nodes = *

[task#1]
type = oci
image = busybox
command = /bin/true
schedule = @1
`))

	// A section with no driver at all has no fallback to answer for.
	assert.NoError(t, create(`
nodes = *

[env]
a = 1

[labels]
b = 2
`))

	// The write that stops a section naming its type is what makes it run the
	// fallback, so that write answers for it.
	const oci = `
nodes = *

[task#1]
type = oci
image = busybox
command = /bin/true
schedule = @1
`
	assert.Error(t, write(t, oci, `
nodes = *

[task#1]
image = busybox
command = /bin/true
schedule = @1
`), "the task dropped its type and runs on the node now")

	// A section that named no type before and names none now runs what it
	// always ran, so an edit elsewhere in it does not answer for the driver.
	assert.NoError(t, write(t, `
nodes = *

[container#0]
image = a
`, `
nodes = *

[container#0]
image = b
`))
}

// A pool ties the storage it serves to the claim the cluster rations it by,
// writing the size of the resource as a reference to the size of the volume.
// Growing the volume is the namespace administrator's to do, and the resource
// size that follows it is not a write of theirs.
func TestAVolumeSizeCarriesWhatPointsAtIt(t *testing.T) {
	volOf := func(s string) *xconfig.T {
		t.Helper()
		p, err := naming.ParsePath("test/vol/foo")
		require.NoError(t, err)
		o, err := object.New(p, object.WithConfigData([]byte(s)), object.WithVolatile(true))
		require.NoError(t, err)
		return o.(object.Configurer).Config()
	}
	volWrite := func(from, to string) error {
		t.Helper()
		return configRbacChanges(admin, naming.KindVol, volOf(from), volOf(to))
	}
	const from = `
nodes = *
pool = loop1
size = 1g

[disk#0]
type = loop
file = /var/lib/opensvc/pool/loop1/foo.img
size = {DEFAULT.size}
`
	assert.NoError(t, volWrite(from, `
nodes = *
pool = loop1
size = 2g

[disk#0]
type = loop
file = /var/lib/opensvc/pool/loop1/foo.img
size = {DEFAULT.size}
`), "the volume grows, and the size of the loop file follows it")

	assert.Error(t, volWrite(from, `
nodes = *
pool = loop1
size = 1g

[disk#0]
type = loop
file = /var/lib/opensvc/pool/loop1/foo.img
size = 2g
`), "the size of the loop file written as a number is a write of the resource")

	assert.Error(t, volWrite(from, `
nodes = *
pool = loop1
size = 2g

[disk#0]
type = loop
file = /var/lib/opensvc/pool/loop1/evil.img
size = {DEFAULT.size}
`), "the file the loop is backed by is not opened by the size pointing at it")
}
