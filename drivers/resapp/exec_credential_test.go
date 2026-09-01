//go:build !windows

package resapp

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aNonRootUser returns a user of the passwd database that is not root,
// to own the script the policy reads an identity from.
func aNonRootUser(t *testing.T) *user.User {
	t.Helper()
	for _, name := range []string{"nobody", "daemon", "bin", "games"} {
		if u, err := user.Lookup(name); err == nil && u.Uid != "0" {
			return u
		}
	}
	t.Skip("no non root user to own the test script")
	return nil
}

// script writes an executable file and gives it to uid:gid, skipping the
// test when the test process may not chown.
func script(t *testing.T, uid, gid int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.sh")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755))
	if err := os.Chown(path, uid, gid); err != nil {
		t.Skipf("chown the test script: %s", err)
	}
	return path
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	i, err := strconv.Atoi(s)
	require.NoError(t, err)
	return i
}

// TestCredentialRunsAScriptAsItsOwner is the v2 behaviour this policy
// exists for: an operator installs a script as themselves, and it runs
// as them, with the object configuration saying nothing about it.
func TestCredentialRunsAScriptAsItsOwner(t *testing.T) {
	owner := aNonRootUser(t)
	path := script(t, atoi(t, owner.Uid), atoi(t, owner.Gid))

	for name, res := range map[string]*T{
		"with no user keyword": {},
		"with a user keyword":  {User: "root"},
		"with a group keyword": {Group: "root"},
		"with both":            {User: "root", Group: "root"},
	} {
		t.Run(name, func(t *testing.T) {
			cred := res.credential([]string{path, "start"})
			assert.Equal(t, owner.Uid, cred.User, "the owner of the script wins")
			assert.Equal(t, owner.Gid, cred.Group, "the group of the script comes with it")
			assert.Equal(t, owner.Username, cred.Name)
		})
	}
}

// TestCredentialOfARootOwnedScript pins the other half: root owns the
// script, so the keywords are what is left to say who runs it.
func TestCredentialOfARootOwnedScript(t *testing.T) {
	path := script(t, 0, 0)
	other := aNonRootUser(t)

	t.Run("with no keyword it is root", func(t *testing.T) {
		cred := (&T{}).credential([]string{path, "start"})
		assert.Equal(t, "root", cred.User)
		assert.Equal(t, "0", cred.Group, "root's primary group, not the daemon's")
	})
	t.Run("the user keyword is honored", func(t *testing.T) {
		cred := (&T{User: other.Username}).credential([]string{path, "start"})
		assert.Equal(t, other.Username, cred.User)
		assert.Equal(t, other.Gid, cred.Group, "the primary group of that user")
		assert.Equal(t, other.Username, cred.Name)
	})
	t.Run("the group keyword overrides the primary group", func(t *testing.T) {
		cred := (&T{User: other.Username, Group: "root"}).credential([]string{path, "start"})
		assert.Equal(t, other.Username, cred.User)
		assert.Equal(t, "root", cred.Group)
	})
}

// TestCredentialOfAScriptOwnedByNobodyKnown covers the uid with no
// passwd entry: v2 called that owner "unknown" and fell back to the
// keywords rather than demoting to a name it could not resolve.
func TestCredentialOfAScriptOwnedByNobodyKnown(t *testing.T) {
	const strayUID = 61234
	if _, err := user.LookupId(strconv.Itoa(strayUID)); err == nil {
		t.Skipf("uid %d is a known user on this host", strayUID)
	}
	path := script(t, strayUID, strayUID)
	cred := (&T{User: "root"}).credential([]string{path, "start"})
	assert.Equal(t, "root", cred.User)
}

func TestCredentialWithoutAFileToRead(t *testing.T) {
	owner := aNonRootUser(t)
	path := script(t, atoi(t, owner.Uid), atoi(t, owner.Gid))

	for name, argv := range map[string][]string{
		// The shell owns /bin/sh, and what it is asked to run is a
		// string the policy cannot stat.
		"a command needing a shell": {"/bin/sh", "-c", path + " start | grep ."},
		// Resolved in PATH at exec time, so there is nothing to stat.
		"a bare command name": {"true"},
		"nothing at all":      {},
	} {
		t.Run(name, func(t *testing.T) {
			cred := (&T{User: "root"}).credential(argv)
			assert.Equal(t, "root", cred.User)
		})
	}
}

func TestCredentialEnv(t *testing.T) {
	assert.Nil(t, execCredential{}.env(), "an unresolved user has no identity to export")
	assert.ElementsMatch(t,
		[]string{"LOGNAME=alice", "USER=alice", "HOME=/home/alice"},
		execCredential{Name: "alice", Home: "/home/alice"}.env())
	assert.ElementsMatch(t,
		[]string{"LOGNAME=alice", "USER=alice"},
		execCredential{Name: "alice"}.env())
}
