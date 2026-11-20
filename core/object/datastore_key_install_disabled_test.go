package object

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

// A disabled volume resource uses no datastore: a change of a key it
// installs leaves its file as it is, as the daemon, which refreshes the
// keys of the datastores the enabled resources use only, does on the
// other nodes.
func TestInstallKeySkipsADisabledVolume(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	svcConf := func(disable bool) map[string]map[string]any {
		return map[string]map[string]any{
			"DEFAULT":        {"nodes": []string{hostname.Hostname()}},
			"volume#enabled": {"name": "enabled", "configs": "store/foo:/cfg"},
			"volume#off":     {"name": "off", "configs": "store/foo:/cfg", "disable": disable},
		}
	}
	svcPath := naming.Path{Name: "client", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	svc, err := NewSvc(svcPath, WithConfigData(svcConf(false)))
	require.NoError(t, err)
	require.NoError(t, svc.Config().Recommit())
	ctx := actioncontext.WithRollbackDisabled(actioncontext.WithLeader(context.Background(), true), true)
	require.NoError(t, svc.Provision(ctx))

	store, err := NewDataStore(naming.Path{Name: "store", Kind: naming.KindCfg, Namespace: naming.NsRoot})
	require.NoError(t, err)
	require.NoError(t, store.AddKey("foo", []byte("v1")))

	installed := func(volName string) string {
		vol, err := NewVol(naming.Path{Name: volName, Kind: naming.KindVol, Namespace: naming.NsRoot})
		require.NoError(t, err)
		b, err := os.ReadFile(filepath.Join(vol.Head(), "cfg"))
		require.NoError(t, err)
		return string(b)
	}
	require.Equal(t, "v1", installed("enabled"))
	require.Equal(t, "v1", installed("off"))

	svc, err = NewSvc(svcPath, WithConfigData(svcConf(true)))
	require.NoError(t, err)
	require.NoError(t, svc.Config().Recommit())

	require.NoError(t, store.ChangeKey("foo", []byte("v2")))
	require.Equal(t, "v2", installed("enabled"), "the enabled volume follows the key")
	require.Equal(t, "v1", installed("off"), "the disabled volume keeps its file")
}
