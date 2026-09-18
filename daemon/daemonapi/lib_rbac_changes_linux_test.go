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
