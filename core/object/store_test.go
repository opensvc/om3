package object

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/hostname"
)

// TestNewStore validates the creation, configuration, provisioning,
// and correctness of keystores and volumes in the system.
func TestNewStore(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	volConf := map[string]map[string]any{
		"DEFAULT": {
			"nodes": []string{hostname.Hostname()},
		},
		"volume#withcfg": {"name": "withcfg", "configs": "store/foo:/cfg"},
		"volume#withsec": {"name": "withsec", "secrets": "store/foo:/sec"},
	}
	clientObjPath := naming.Path{Name: "client", Kind: naming.KindSvc, Namespace: "root"}
	t.Logf("prepare %s object that will setup the volumes with configs and secrets", clientObjPath)
	t.Logf("creating %s object %s", clientObjPath, clientObjPath.ConfigFile())
	clientObj, err := NewSvc(clientObjPath, WithConfigData(volConf))
	assert.NoError(t, err)

	t.Logf("commit %s object", clientObjPath)
	assert.NoError(t, clientObj.Config().Recommit())

	t.Logf("provision %s object to create the volumes", clientObjPath)
	provisionCtx := context.Background()
	provisionCtx = actioncontext.WithLeader(provisionCtx, true)
	provisionCtx = actioncontext.WithRollbackDisabled(provisionCtx, true)
	assert.NoError(t, clientObj.Provision(provisionCtx))

	for _, c := range []naming.Kind{naming.KindCfg, naming.KindSec} {
		t.Run(c.String(), func(t *testing.T) {
			storePath := naming.Path{Name: "store", Kind: c, Namespace: "root"}
			t.Logf("%s create config: %s", storePath, storePath.ConfigFile())
			o, err := NewKeystore(storePath)
			require.NoError(t, err)

			t.Logf("%s add key foo", storePath)
			require.NoError(t, o.AddKey("foo", []byte(fmt.Sprintf("value of %s", c))))

			t.Logf("%s installKey", storePath)
			require.NoError(t, o.InstallKey("demo"))

			volPath := naming.Path{Name: "with" + c.String(), Namespace: "root", Kind: naming.KindVol}
			t.Logf("check installed key on %s head", volPath)
			var vol Vol
			vol, err = NewVol(volPath)
			require.NoError(t, err)

			installedFile := fmt.Sprintf("%s/%s", vol.Head(), c)
			var b []byte
			b, err = os.ReadFile(installedFile)
			require.NoError(t, err, "installed file %s should exist", installedFile)
			require.Equal(t, fmt.Sprintf("value of %s", c), string(b),
				"installed file %s should contain value of %s",
				installedFile, c)
		})
	}
}
