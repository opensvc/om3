package daemondata_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/cmd"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	b, err := os.ReadFile(path)
	require.Nil(t, err)
	return b
}

func LoadFull(t *testing.T, name string) *node.Node {
	t.Helper()
	var full node.Node
	require.Nil(t, json.Unmarshal(loadFixture(t, name), &full))
	return &full
}

func LoadPatch(t *testing.T, name string) *hbtype.Msg {
	t.Helper()
	var msg hbtype.Msg
	require.Nil(t, json.Unmarshal(loadFixture(t, name), &msg))
	return &msg
}

func TestMain(m *testing.M) {
	testhelper.Main(m, cmd.ExecuteArgs)
}

func setup(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/cluster-2-nodes.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca-cluster1.conf")
	env.InstallFile("../../testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert-cluster1.conf")
	rawconfig.LoadSections()
}

// TestDaemonData runs sequence of data updates withing t.Run, and fail fast on
// first error
//
// This is why each t.Run is followed by require.False(t, t.Failed()) // fail on first error
func TestDaemonData(t *testing.T) {
	setup(t)

	drainDuration := 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Logf("start daemon bus")
	psbus := pubsub.NewBus("daemon")
	psbus.Start(ctx)
	ctx = pubsub.ContextWithBus(ctx, psbus)
	defer psbus.Stop()

	t.Logf("start daemondata")
	cmdC, hbRecvMsgQ, cancel := daemondata.Start(ctx, 10*time.Millisecond)
	defer cancel()

	ctx = daemondata.ContextWithBus(ctx, cmdC)
	ctx = daemonctx.WithHBRecvMsgQ(ctx, hbRecvMsgQ)

	require.NoError(t, ccfg.Start(ctx, drainDuration))

	bus := daemondata.New(cmdC)
	localNode := hostname.Hostname()
	remoteHost := "node2"

	t.Run("from daemon start", func(t *testing.T) {
		t.Run("GetStatus return status with instance state initialized", func(t *testing.T) {
			localNodeStatus := bus.GetNodeStatus(localNode)
			require.NotNil(t, localNodeStatus)
			require.Equalf(t, uint64(1), localNodeStatus.Gen[localNode],
				"expected local node gen 1, got %+v", localNodeStatus)
			cluster := bus.GetStatus().Cluster
			require.Equal(t, "cluster1", cluster.Config.Name)
			require.Equalf(t, "d0cdc684-b235-11eb-b929-acde48001122", cluster.Config.ID,
				"got %+v", cluster)
		})
		require.False(t, t.Failed()) // fail on first error

		t.Run("GetNode return node data with local data initialized", func(t *testing.T) {
			localNode := bus.GetNode(localNode)
			require.Equalf(t, node.MonitorStateIdle, localNode.Monitor.State,
				"got %+v", localNode)
		})
		require.False(t, t.Failed()) // fail on first error
	})
	require.False(t, t.Failed()) // fail on first error

	t.Run("Ensure GetNode result is a deep copy", func(t *testing.T) {
		initial := bus.GetNode(localNode)
		initial.Monitor.State = node.MonitorStateIdle
		initial.Status.Gen[localNode] = 30
		refreshed := bus.GetNode(localNode)
		assert.NotNil(t, refreshed)
		assert.Equal(t, uint64(1), refreshed.Status.Gen[localNode])
		assert.Equal(t, node.MonitorStateIdle, refreshed.Monitor.State)
	})
	require.False(t, t.Failed()) // fail on first error

	t.Run("Ensure GetNodeMonitor result is a deep copy", func(t *testing.T) {
		initial := bus.GetNodeMonitor(localNode)
		initialUpdated := initial.StateUpdated
		initialGlobalExpectUpdated := initial.GlobalExpectUpdated
		initial.State = node.MonitorStateIdle
		initial.StateUpdated = time.Now()
		initial.GlobalExpect = node.MonitorGlobalExpectAborted
		initial.GlobalExpectUpdated = time.Now()

		refreshed := bus.GetNodeMonitor(localNode)
		require.Equal(t, node.MonitorStateIdle, refreshed.State, "State changed !")
		require.Equal(t, initialUpdated, refreshed.StateUpdated, "StateUpdated changed !")
		require.Equal(t, node.MonitorGlobalExpectNone, refreshed.GlobalExpect, "GlobalExpect changed !")
		require.Equal(t, initialGlobalExpectUpdated, refreshed.GlobalExpectUpdated, "GlobalExpectUpdated changed !")
	})
	require.False(t, t.Failed()) // fail on first error

	t.Run("on receive hb messages...", func(t *testing.T) {
		t.Run("on receive hb message full-node2-t1.json", func(t *testing.T) {
			full := LoadFull(t, "full-node2-t1.json")
			fullGens := make(map[string]uint64)
			for n, gen := range full.Status.Gen {
				fullGens[n] = gen
			}
			msg := hbtype.Msg{
				Kind:     "full",
				Gen:      fullGens,
				Full:     *full,
				Nodename: remoteHost,
			}
			hbRecvMsgQ <- &msg

			nodeLocal := bus.GetNode(remoteHost)
			t.Log("check cluster local gens view of remote")
			require.Equal(t, full.Status.Gen[remoteHost], nodeLocal.Status.Gen[remoteHost], "local node gens has not been updated with remote gen value")

			nodeRemote := bus.GetNode(remoteHost)
			t.Log("check remote node gens")
			require.Equal(t, full.Status.Gen, nodeRemote.Status.Gen, "remote status gens are not gens from message")
			t.Log("check remote node instance status")
			require.Equal(t, full.Instance["foo"].Status.Updated, nodeRemote.Instance["foo"].Status.Updated, "instance status updated mismatch")
			t.Log("check remote node instance monitor")
			require.Equal(t, instance.MonitorStateStarting, nodeRemote.Instance["foo"].Monitor.State, "instance monitor state mismatch")
			t.Log("check remote node stats monitor")
			require.Equal(t, 0.4, nodeRemote.Stats.Load15M)
			require.Equal(t, uint64(16012), nodeRemote.Stats.MemTotalMB)
			require.Equal(t, uint64(96), nodeRemote.Stats.MemAvailPct)
			require.Equal(t, uint64(979), nodeRemote.Stats.SwapTotalMB)
		})

		t.Run("on receive hb message from non cluster member", func(t *testing.T) {
			peerNotMemmber := "peer-not-member"
			full := LoadFull(t, "full-node2-t1.json")
			fullGens := make(map[string]uint64)
			for n, gen := range full.Status.Gen {
				fullGens[n] = gen
			}
			msg := hbtype.Msg{
				Kind:     "full",
				Gen:      fullGens,
				Full:     *full,
				Nodename: peerNotMemmber,
			}
			hbRecvMsgQ <- &msg

			assert.Nilf(t, bus.GetNode(peerNotMemmber),
				"not cluster member '%s' message should not be applied", peerNotMemmber)
			nodeLocal := bus.GetNode(localNode)
			notPeerGens, ok := nodeLocal.Status.Gen[peerNotMemmber]
			assert.Falsef(t, ok, "not cluster member has been added to local status gens: %v", notPeerGens)
		})
		require.False(t, t.Failed()) // fail on first error

		t.Run("on receive hb message patch...", func(t *testing.T) {
			t.Run("patch-node2-t2.json", func(t *testing.T) {
				patchMsg := LoadPatch(t, "patch-node2-t2.json")
				hbRecvMsgQ <- patchMsg

				nodeLocal := bus.GetNode(localNode)
				require.Equal(t, patchMsg.Gen[remoteHost], nodeLocal.Status.Gen[remoteHost], "local node gens has not been updated with remote gen value")

				nodeRemote := bus.GetNode(remoteHost)
				require.NotNil(t, nodeRemote)
				require.Equal(t, patchMsg.Gen, nodeRemote.Status.Gen, "remote status gens are not gens from message")
				require.Equal(t, 0.5, nodeRemote.Stats.Load15M)
				require.Equal(t, uint64(1000), nodeRemote.Stats.MemTotalMB)
				require.Equal(t, uint64(10), nodeRemote.Stats.MemAvailPct)
				require.Equal(t, uint64(11), nodeRemote.Stats.SwapTotalMB)
			})
			require.False(t, t.Failed()) // fail on first error

			t.Run("patch with some already applied gens gen patch-node2-t3-with-t2-changed.json", func(t *testing.T) {
				assert.Equal(t, instance.MonitorStateStarting, bus.GetNode(remoteHost).Instance["foo"].Monitor.State)
				patchMsg := LoadPatch(t, "patch-node2-t3-with-t2-changed.json")
				hbRecvMsgQ <- patchMsg

				remoteNode := bus.GetNode(remoteHost)
				assert.Equal(t, 0.5, remoteNode.Stats.Load15M, "hum hacked gen 21 has been reapplied !")
				assert.Equal(t, uint64(2), remoteNode.Stats.Score, "hum gen 22 has not been applied !")
			})
			require.False(t, t.Failed()) // fail on first error

			t.Run("broken gen sequence patch-node2-t4.json", func(t *testing.T) {
				patchMsg := LoadPatch(t, "patch-node2-t4.json")
				hbRecvMsgQ <- patchMsg

				localNode := bus.GetNode(localNode)
				assert.Equal(t, uint64(0), localNode.Status.Gen[remoteHost], "expect local node needs full from remote")

				t.Log("ensure future delta not applied")
				remoteNode := bus.GetNode(remoteHost)
				require.NotNil(t, remoteNode)
				require.Equal(t, uint64(2), bus.GetNode(remoteHost).Stats.Score, "hum some remote data should has been applied !")

			})
			require.False(t, t.Failed()) // fail on first error
		})
		require.False(t, t.Failed()) // fail on first error

		t.Run("verify cluster schema", func(t *testing.T) {
			cluster := bus.GetStatus().Cluster

			// cluster.node.<node>.config
			require.Equal(t, "cluster1", cluster.Config.Name)
			// TODO ensure expected cluster id from test cluster.conf file
			require.Equal(t, "d0cdc684-b235-11eb-b929-acde48001122", cluster.Config.ID)
			//require.Equal(t, []string{"node1", "node2"}, cluster.Config.Nodes)

			// cluster.node.<node>.status
			require.Equal(t, false, cluster.Status.Compat)

			// instance
			remoteNodeInstanceX := cluster.Node["node2"].Instance["foo"]
			require.Equal(t, status.Down, remoteNodeInstanceX.Status.Avail)
			require.Equal(t, instance.MonitorStateStarting, remoteNodeInstanceX.Monitor.State)
		})
		require.False(t, t.Failed()) // fail on first error

		t.Run("bus count stats", func(t *testing.T) {
			for name, count := range bus.Stats() {
				require.Greaterf(t, count, uint64(0), "expect %s count > 0", name)
				t.Logf("cout %s: %d", name, count)
			}
		})
	})
}
