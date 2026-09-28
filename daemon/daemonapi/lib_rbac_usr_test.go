package daemonapi

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// withClusterConfig sets the cluster configuration a usr object sets up its
// encryption from, and puts back the one the other tests run with.
func withClusterConfig(t *testing.T) {
	t.Helper()
	if cluster.ConfigData.IsSet() {
		prev := cluster.ConfigData.Get()
		t.Cleanup(func() { cluster.ConfigData.Set(prev) })
	} else {
		t.Cleanup(cluster.InitData)
	}
	c := &cluster.Config{Name: "test"}
	c.SetSecret("0123456789abcdef0123456789abcdef")
	cluster.ConfigData.Set(c)
}

func usrConfig(t *testing.T, s string) *xconfig.T {
	t.Helper()
	p, err := naming.ParsePath("system/usr/u1")
	require.NoError(t, err)
	o, err := object.New(p, object.WithConfigData([]byte(s)), object.WithVolatile(true))
	require.NoError(t, err)
	return o.(object.Configurer).Config()
}

// A user is its grants, so a writer gives a user only what it holds itself,
// and the name a user authenticates by with a certificate is root's to set.
func TestUsrRbac(t *testing.T) {
	withClusterConfig(t)
	p, _ := naming.ParsePath("system/usr/u1")
	sysAdmin := rbac.NewGrants("admin:system")
	for _, tc := range []struct {
		name    string
		from    string
		to      string
		refused bool
	}{
		{"make a user granted root", "", "[DEFAULT]\ngrant = root\n", true},
		{"make a user granted what the writer holds", "", "[DEFAULT]\ngrant = admin:system\n", false},
		{"make a user granted another namespace", "", "[DEFAULT]\ngrant = admin:ns1\n", true},
		{"add root to a user", "[DEFAULT]\ngrant = guest:system\n", "[DEFAULT]\ngrant = guest:system root\n", true},
		{"edit a root user without adding grants", "[DEFAULT]\ngrant = root\n", "[DEFAULT]\ngrant = root\ncomment = x\n", false},
		{"take root away from a user", "[DEFAULT]\ngrant = root admin:system\n", "[DEFAULT]\ngrant = admin:system\n", false},
		{"set the cn of a user", "[DEFAULT]\ngrant = admin:system\n", "[DEFAULT]\ngrant = admin:system\ncn = u2\n", true},
		{"keep the cn of a user", "[DEFAULT]\ncn = u1\n", "[DEFAULT]\ncn = u1\ncomment = x\n", false},
		{"make a user with its own name as cn", "", "[DEFAULT]\ncn = u1\n", false},
		{"make a user with the cn of another", "", "[DEFAULT]\ncn = root\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var from *xconfig.T
			if tc.from != "" {
				from = usrConfig(t, tc.from)
			}
			err := usrRbac(sysAdmin, p, from, usrConfig(t, tc.to))
			if tc.refused {
				assert.True(t, errors.Is(err, ErrDenied), "%v", err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
	svc, _ := naming.ParsePath("ns1/svc/s1")
	assert.NoError(t, usrRbac(sysAdmin, svc, nil, usrConfig(t, "[DEFAULT]\ngrant = root\n")), "not a user")
}
