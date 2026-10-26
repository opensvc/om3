package omcmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
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
	objects := map[string]object.Status{
		"svc1":         {Scope: []string{"node1", "node2"}},
		"ns1/svc/svc2": {Scope: []string{"node2"}},
		"ns1/vol/vol1": {Scope: []string{"node1", "node2"}},
		"ns1/vol/vol2": {Scope: []string{"node1"}},
		"ns1/cfg/cfg1": {Scope: []string{"node2"}},
		"ns1/svc/svc3": {Scope: []string{"node1"}},
	}
	installed := naming.Paths{mustParse("svc1"), mustParse("ns1/vol/vol1")}
	assert.Equal(t, []string{"ns1/svc/svc2"}, notInstalledObjects(objects, installed, "node2"))
	assert.Equal(t, []string{"ns1/svc/svc3", "ns1/vol/vol2"}, notInstalledObjects(objects, installed, "node1"))
	assert.Empty(t, notInstalledObjects(objects, installed, "node3"))
}
