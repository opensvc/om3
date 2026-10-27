package omcmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/key"
)

// TestCollectorRegistrationSurvivesTheJoin pins that the registration id
// read before the join moves the node config aside, and the one stored after
// it in the fresh node config, are the node.uuid keyword ones.
func TestCollectorRegistrationSurvivesTheJoin(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte("[node]\nuuid = uuid-old\n"), 0600))
	assert.Equal(t, "uuid-old", localNodeUUID())

	// The join moves the node config aside.
	require.NoError(t, os.Remove(rawconfig.NodeConfigFile()))
	assert.Empty(t, localNodeUUID())

	require.NoError(t, storeCollectorRegistration("uuid-new"))
	n, err := object.NewNode()
	require.NoError(t, err)
	assert.Equal(t, "uuid-new", n.Config().GetString(key.Parse("node.uuid")))
}

// TestNotInstalledObjects pins the objects the join waits for before the
// checks and disks pushes: the svc and vol scoped on the node, not
// installed yet.
func TestNotInstalledObjects(t *testing.T) {
	mustParse := func(s string) naming.Path {
		p, err := naming.ParsePath(s)
		require.NoError(t, err)
		return p
	}
	scopes := map[string][]string{
		"svc1":         {"node1", "node2"},
		"ns1/svc/svc2": {"node2"},
		"ns1/vol/vol1": {"node1", "node2"},
		"ns1/vol/vol2": {"node1"},
		"ns1/cfg/cfg1": {"node2"},
		"ns1/svc/svc3": {"node1"},
	}
	installed := naming.Paths{mustParse("svc1"), mustParse("ns1/vol/vol1")}
	assert.Equal(t, []string{"ns1/svc/svc2"}, notInstalledObjects(scopes, installed, "node2"))
	assert.Equal(t, []string{"ns1/svc/svc3", "ns1/vol/vol2"}, notInstalledObjects(scopes, installed, "node1"))
	assert.Empty(t, notInstalledObjects(scopes, installed, "node3"))
}

// TestObjectScopes pins that an object a peer has the instance configuration
// of is waited for before the object monitors gave it a status.
func TestObjectScopes(t *testing.T) {
	var data clusterdump.Data
	data.Cluster.Object = map[string]object.Status{
		"svc1": {Scope: []string{"node1", "node2"}},
	}
	var peer node.Node
	peer.Instance = map[string]instance.Instance{
		"svc1":         {Config: &instance.Config{Scope: []string{"node1", "node2"}}},
		"ns1/svc/svc2": {Config: &instance.Config{Scope: []string{"node1", "node2"}}},
		"ns1/svc/svc3": {},
	}
	data.Cluster.Node = map[string]node.Node{"node1": peer}

	scopes := objectScopes(data)
	assert.Contains(t, scopes["svc1"], "node2")
	assert.Equal(t, []string{"node1", "node2"}, scopes["ns1/svc/svc2"], "no status yet")
	assert.Empty(t, scopes["ns1/svc/svc3"], "no config")
	assert.Equal(t, []string{"ns1/svc/svc2", "svc1"}, notInstalledObjects(scopes, nil, "node2"))
}
