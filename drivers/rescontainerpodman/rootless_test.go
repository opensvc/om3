package rescontainerpodman

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The subordinate id files are read the way shadow-utils reads them: a range
// is granted to a user by name or by uid, and an empty range grants nothing.
func TestHasSubordinateIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subuid")
	require.NoError(t, os.WriteFile(path, []byte("alice:100000:65536\n1002:200000:65536\nbob:300000:0\n\n# comment\n"), 0644))

	for _, tc := range []struct {
		name     string
		uid      uint32
		expected bool
		says     string
	}{
		{"alice", 1001, true, "a range granted by name"},
		{"carol", 1002, true, "a range granted by uid"},
		{"bob", 1003, false, "an empty range"},
		{"dave", 1004, false, "no range"},
	} {
		ok, err := hasSubordinateIDs(path, tc.name, tc.uid)
		require.NoError(t, err)
		assert.Equalf(t, tc.expected, ok, "%s: %s", tc.name, tc.says)
	}

	ok, err := hasSubordinateIDs(filepath.Join(dir, "missing"), "alice", 1001)
	require.NoError(t, err, "a node with no subordinate id file")
	assert.False(t, ok)
}

// What the node lacks is refused by name, with the command that fixes it,
// rather than surfacing as a podman error at the next boot.
func TestCheckNamesWhatTheNodeLacks(t *testing.T) {
	dir := t.TempDir()
	defer func(a, b, c string) { subuidFile, subgidFile, runtimeDirRoot = a, b, c }(subuidFile, subgidFile, runtimeDirRoot)
	subuidFile = filepath.Join(dir, "subuid")
	subgidFile = filepath.Join(dir, "subgid")
	runtimeDirRoot = filepath.Join(dir, "run", "user")
	u := rootlessUser{Name: "alice", UID: 1001, GID: 1001, Home: "/home/alice"}

	err := u.check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loginctl enable-linger alice")
	assert.Contains(t, err.Error(), "no subordinate ids in "+subuidFile)
	assert.Contains(t, err.Error(), "no subordinate ids in "+subgidFile)
	assert.Len(t, flatten(err), 3, "one status line per missing setup")

	require.NoError(t, os.MkdirAll(filepath.Join(runtimeDirRoot, "1001"), 0700))
	require.NoError(t, os.WriteFile(subuidFile, []byte("alice:100000:65536\n"), 0644))
	require.NoError(t, os.WriteFile(subgidFile, []byte("alice:100000:65536\n"), 0644))
	assert.NoError(t, u.check(), "a node set up for the user")
}

// Podman finds the store and the runtime directory of the user in the
// environment, and the agent's own XDG directories must not lead it to
// root's.
func TestCredentialIsTheUsersIdentityAndEnvironment(t *testing.T) {
	u := rootlessUser{Name: "alice", UID: 1001, GID: 2000, Home: "/home/alice"}
	c := u.credential()
	assert.Equal(t, uint32(1001), c.UID)
	assert.Equal(t, uint32(2000), c.GID)
	assert.Equal(t, "/home/alice", c.Home)
	for _, v := range []string{
		"HOME=/home/alice",
		"USER=alice",
		"LOGNAME=alice",
		"XDG_RUNTIME_DIR=/run/user/1001",
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus",
		"XDG_CONFIG_HOME=/home/alice/.config",
		"XDG_DATA_HOME=/home/alice/.local/share",
		"XDG_CACHE_HOME=/home/alice/.cache",
	} {
		assert.Contains(t, c.Env, v)
	}
}

// The groups are placed in the subtree systemd delegates to the user, which
// is where their systemd instance places the container.
func TestDelegationIsTheSubtreeOfTheUsersSystemdInstance(t *testing.T) {
	u := rootlessUser{Name: "alice", UID: 1001, GID: 2000}
	d := u.delegation()
	assert.Equal(t, "/user.slice/user-1001.slice/user@1001.service", d.Root)
	assert.Equal(t, 1001, d.UID)
	assert.Equal(t, 2000, d.GID)
}

// A rootless container is run by an unprivileged user: naming root is a
// configuration error, not a request for a rootful container.
func TestRootlessUserRefusesRootAndUnknownUsers(t *testing.T) {
	_, err := (&T{RootlessUser: "root"}).rootlessUser()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is root")

	_, err = (&T{RootlessUser: "omtest-no-such-user"}).rootlessUser()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rootless_user omtest-no-such-user")

	u, err := (&T{}).rootlessUser()
	require.NoError(t, err)
	assert.Nil(t, u, "no rootless_user is a rootful container")
}

// Only a rootless container has its commands demoted: a rootful one keeps
// running podman as the agent.
func TestExecutorArgCredentialFollowsRootlessUser(t *testing.T) {
	d := &T{}
	require.NoError(t, d.Configure())
	c, err := d.executorArg().Credential()
	require.NoError(t, err)
	assert.Nil(t, c)
	assert.Nil(t, d.executorArg().ExecutorArg.ResolvConfDir, "a rootful container writes its resolver in its var dir")

	d = &T{RootlessUser: "nobody"}
	c, err = d.executorArg().Credential()
	require.NoError(t, err)
	if assert.NotNil(t, c) {
		assert.Equal(t, uint32(65534), c.UID)
	}
	assert.NotNil(t, d.executorArg().ExecutorArg.ResolvConfDir, "a rootless one writes it where its user can read it")
}
