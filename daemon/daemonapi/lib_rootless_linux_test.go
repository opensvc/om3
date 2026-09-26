//go:build linux

package daemonapi

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

// allowRootless writes the configuration of the test namespace, allowing the
// users given to run rootless containers.
func allowRootless(t *testing.T, users string) {
	t.Helper()
	knownNodes(t)
	p := naming.Path{Namespace: "test", Kind: naming.KindNscfg, Name: "namespace"}
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nrootless_users = "+users+"\n"), 0644))
}

func rootlessWrite(t *testing.T, from, to string) error {
	t.Helper()
	p, _ := naming.ParsePath("test/svc/foo")
	if from == "" {
		return rootlessRbac(p, nil, configOf(t, to))
	}
	return rootlessRbac(p, configOf(t, from), configOf(t, to))
}

const podman = "[DEFAULT]\nnodes = node1\n[container#1]\ntype = podman\nimage = nginx\n"

func TestARootlessAccountMustBeAllowedByTheNamespace(t *testing.T) {
	allowRootless(t, "nobody")
	assert.NoError(t, rootlessWrite(t, "", podman+"rootless_user = nobody\n"))
	err := rootlessWrite(t, "", podman+"rootless_user = daemon\n")
	assert.True(t, errors.Is(err, ErrDenied), "%v", err)
	assert.Error(t, rootlessWrite(t, "", podman+"rootless_user = nobody\nrootless_group = disk\n"))
	assert.NoError(t, rootlessWrite(t, "", podman), "a rootful container is not an account of the namespace")
}

// The account is judged as it evaluates: through a reference, and on the node
// a scoped value names it for.
func TestARootlessAccountIsJudgedAsItEvaluates(t *testing.T) {
	allowRootless(t, "nobody")
	assert.Error(t, rootlessWrite(t, "", podman+"rootless_user = {env.u}\n[env]\nu = daemon\n"))
	assert.Error(t, rootlessWrite(t, "", podman+"rootless_user = nobody\nrootless_user@node1 = daemon\n"))

	// Moving what the reference reads is a change of the account.
	from := podman + "rootless_user = {env.u}\n[env]\nu = nobody\n"
	assert.NoError(t, rootlessWrite(t, from, from))
	assert.Error(t, rootlessWrite(t, from, podman+"rootless_user = {env.u}\n[env]\nu = daemon\n"))
}

// A write that leaves the account as it was is not judged on it: the squatter
// may have stopped allowing an account after the object was written, and the
// object is still edited.
func TestAnAccountTheWriteDoesNotChangeIsNotJudged(t *testing.T) {
	allowRootless(t, "nobody")
	from := podman + "rootless_user = daemon\n"
	assert.NoError(t, rootlessWrite(t, from, from+"[env]\nfoo = bar\n"))
}
