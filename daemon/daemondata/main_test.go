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

	"opensvc.com/opensvc/cmd"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/testhelper"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	b, err := os.ReadFile(path)
	require.Nil(t, err)
	return b
}

func LoadFull(t *testing.T, name string) *cluster.NodeData {
	t.Helper()
	var full cluster.NodeData
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
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Logf("start daemon bus")
	psbus := pubsub.NewBus("daemon")
	psbus.Start(ctx)
	ctx = pubsub.ContextWithBus(ctx, psbus)
	defer psbus.Stop()

	t.Logf("start daemondata")
	cmdC, hbRecvMsgQ, cancel := daemondata.Start(ctx)
	defer cancel()

	bus := daemondata.New(cmdC)
	localNode := hostname.Hostname()
	remoteHost := "node2"

	t.Run("from initialized", func(t *testing.T) {
		t.Run("GetStatus return status with local status initialized", func(t *testing.T) {
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

		t.Run("GetNodeData return node data with local data initialized", func(t *testing.T) {
			localNodeData := bus.GetNodeData(localNode)
			require.Equalf(t, cluster.NodeMonitorStateInit, localNodeData.Monitor.State,
				"got %+v", localNodeData)
		})
		require.False(t, t.Failed()) // fail on first error
	})
	require.False(t, t.Failed()) // fail on first error

	t.Run("Ensure GetNodeData result is a deep copy", func(t *testing.T) {
		initialData := bus.GetNodeData(localNode)
		initialData.Monitor.State = cluster.NodeMonitorStateIdle
		initialData.Status.Gen[localNode] = 30
		refreshedData := bus.GetNodeData(localNode)
		assert.NotNil(t, refreshedData)
		assert.Equal(t, uint64(1), refreshedData.Status.Gen[localNode])
		assert.Equal(t, cluster.NodeMonitorStateInit, refreshedData.Monitor.State)
	})
	require.False(t, t.Failed()) // fail on first error

	t.Run("Ensure GetNodeMonitor result is a deep copy", func(t *testing.T) {
		initialData := bus.GetNodeMonitor(localNode)
		initialDataUpdated := initialData.StateUpdated
		initialDataGlobalExpectUpdated := initialData.GlobalExpectUpdated
		initialData.State = cluster.NodeMonitorStateIdle
		initialData.StateUpdated = time.Now()
		initialData.GlobalExpect = cluster.NodeMonitorGlobalExpectAborted
		initialData.GlobalExpectUpdated = time.Now()

		refreshedData := bus.GetNodeMonitor(localNode)
		require.Equal(t, cluster.NodeMonitorStateInit, refreshedData.State, "State changed !")
		require.Equal(t, initialDataUpdated, refreshedData.StateUpdated, "StateUpdated changed !")
		require.Equal(t, cluster.NodeMonitorGlobalExpectUnset, refreshedData.GlobalExpect, "GlobalExpect changed !")
		require.Equal(t, initialDataGlobalExpectUpdated, refreshedData.GlobalExpectUpdated, "GlobalExpectUpdated changed !")
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

			nodeLocal := bus.GetNodeData(remoteHost)
			t.Log("check cluster local gens view of remote")
			require.Equal(t, full.Status.Gen[remoteHost], nodeLocal.Status.Gen[remoteHost], "local node gens has not been updated with remote gen value")

			nodeRemote := bus.GetNodeData(remoteHost)
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
		require.False(t, t.Failed()) // fail on first error

		t.Run("on receive hb message patch...", func(t *testing.T) {
			t.Run("patch-node2-t2.json", func(t *testing.T) {
				patchMsg := LoadPatch(t, "patch-node2-t2.json")
				hbRecvMsgQ <- patchMsg

				nodeLocal := bus.GetNodeData(localNode)
				require.Equal(t, patchMsg.Gen[remoteHost], nodeLocal.Status.Gen[remoteHost], "local node gens has not been updated with remote gen value")

				nodeRemote := bus.GetNodeData(remoteHost)
				require.NotNil(t, nodeRemote)
				require.Equal(t, patchMsg.Gen, nodeRemote.Status.Gen, "remote status gens are not gens from message")
				require.Equal(t, 0.5, nodeRemote.Stats.Load15M)
				require.Equal(t, uint64(1000), nodeRemote.Stats.MemTotalMB)
				require.Equal(t, uint64(10), nodeRemote.Stats.MemAvailPct)
				require.Equal(t, uint64(11), nodeRemote.Stats.SwapTotalMB)
			})
			require.False(t, t.Failed()) // fail on first error

			t.Run("patch with some already applied gens gen patch-node2-t3-with-t2-changed.json", func(t *testing.T) {
				assert.Equal(t, instance.MonitorStateStarting, bus.GetNodeData(remoteHost).Instance["foo"].Monitor.State)
				patchMsg := LoadPatch(t, "patch-node2-t3-with-t2-changed.json")
				hbRecvMsgQ <- patchMsg

				remoteNodeData := bus.GetNodeData(remoteHost)
				assert.Equal(t, 0.5, remoteNodeData.Stats.Load15M, "hum hacked gen 21 has been reapplied !")
				assert.Equal(t, uint64(2), remoteNodeData.Stats.Score, "hum gen 22 has not been applied !")
			})
			require.False(t, t.Failed()) // fail on first error

			t.Run("broken gen sequence patch-node2-t4.json", func(t *testing.T) {
				patchMsg := LoadPatch(t, "patch-node2-t4.json")
				hbRecvMsgQ <- patchMsg

				localNodeData := bus.GetNodeData(localNode)
				assert.Equal(t, uint64(0), localNodeData.Status.Gen[remoteHost], "expect local node needs full from remote")

				t.Log("ensure future delta not applied")
				remoteNodeData := bus.GetNodeData(remoteHost)
				require.NotNil(t, remoteNodeData)
				require.Equal(t, uint64(2), bus.GetNodeData(remoteHost).Stats.Score, "hum some remote data should has been applied !")

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
