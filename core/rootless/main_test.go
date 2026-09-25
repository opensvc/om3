//go:build linux

package rootless

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

// namespace writes the configuration of a namespace listing users and groups.
func namespace(t *testing.T, name, users, groups string) {
	t.Helper()
	p := naming.Path{Namespace: name, Kind: naming.KindNscfg, Name: "namespace"}
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
	s := "[DEFAULT]\n"
	if users != "" {
		s += "rootless_users = " + users + "\n"
	}
	if groups != "" {
		s += "rootless_groups = " + groups + "\n"
	}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(s), 0644))
}

func load(t *testing.T, name string) Allowed {
	t.Helper()
	a, err := Load(name)
	require.NoError(t, err)
	return a
}

func TestAccountsAreAllowedByTheirNamespace(t *testing.T) {
	testhelper.Setup(t)
	namespace(t, "ci", "nobody", "")
	a := load(t, "ci")

	assert.NoError(t, a.Check("", ""), "a rootful container is not an account of the namespace")
	assert.NoError(t, a.Check("nobody", ""), "the primary group of an allowed user is allowed")
	assert.NoError(t, a.Check("65534", "nogroup"), "the ids are compared, not the names")
	assert.Error(t, a.Check("daemon", ""), "an account the namespace does not list")
	assert.Error(t, a.Check("nobody", "disk"), "a group the namespace does not list")
}

func TestAListedGroupIsAllowed(t *testing.T) {
	testhelper.Setup(t)
	namespace(t, "ci", "nobody", "disk")
	assert.NoError(t, load(t, "ci").Check("nobody", "disk"))
}

// The root account and group reach everything, whatever a namespace says.
func TestRootIsNeverAllowed(t *testing.T) {
	testhelper.Setup(t)
	namespace(t, "ci", "root nobody", "root")
	a := load(t, "ci")
	assert.Error(t, a.Check("root", ""))
	assert.Error(t, a.Check("0", ""))
	assert.Error(t, a.Check("nobody", "root"))
	assert.Error(t, a.Check("nobody", "0"))
}

func TestANamespaceListingNothingAllowsNothing(t *testing.T) {
	testhelper.Setup(t)
	assert.Error(t, load(t, "unconfigured").Check("nobody", ""))
	namespace(t, "ci", "", "")
	assert.Error(t, load(t, "ci").Check("nobody", ""))
}

// A node judging a write need not have the accounts of the object: it judges
// the name the namespace lists.
func TestAnAccountThisNodeLacksIsJudgedByName(t *testing.T) {
	testhelper.Setup(t)
	namespace(t, "ci", "omtest-no-such-account", "")
	a := load(t, "ci")
	assert.NoError(t, a.Check("omtest-no-such-account", ""))
	assert.Error(t, a.Check("omtest-another-missing-account", ""))
}

func TestAnAccountAllowedInTwoNamespacesIsShared(t *testing.T) {
	testhelper.Setup(t)
	namespace(t, "ci", "nobody", "")
	namespace(t, "other", "65534", "")
	namespace(t, "third", "daemon", "")
	shared, err := load(t, "ci").SharedWith()
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{"65534": {"other"}}, shared)
	assert.Contains(t, DescribeShared(shared), "nobody is also allowed in other")
}
