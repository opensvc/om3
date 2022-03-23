package daemondatatest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/util/hostname"
)

func TestDaemonData(t *testing.T) {
	var (
		tName                             string
		status                            *cluster.Status
		localNodeStatus, remoteNodeStatus *cluster.NodeStatus
		err                               error
	)
	_ = err
	cmdC, cancel := daemondata.Start(context.Background())
	defer cancel()
	bus := daemondata.New(cmdC)
	localNode := hostname.Hostname()
	remoteHost := "node2"

	tName = "getLocalNodeStatus return status with local status initialized"
	t.Run(tName, func(t *testing.T) {
		localNodeStatus = bus.GetLocalNodeStatus()
		require.Equal(t, localNodeStatus.Gen[localNode], uint64(0))
		require.Equal(t, localNodeStatus.Monitor.Status, "")
	})

	tName = "GetStatus return status with local status initialized"
	t.Run(tName, func(t *testing.T) {
		status = bus.GetStatus()
		localNodeStatus = daemondata.GetNodeStatus(status, localNode)
		require.NotNil(t, localNodeStatus)
		require.Equal(t, uint64(0), localNodeStatus.Gen[localNode])
		require.Equal(t, "", localNodeStatus.Monitor.Status)

		tName = "updates on GetStatus result are local"
		t.Run(tName, func(t *testing.T) {
			localNodeStatus.Monitor.Status = "foo"
			localNodeStatus.Gen[localNode] = 1
			status = bus.GetStatus()
			localNodeStatus = daemondata.GetNodeStatus(status, localNode)
			require.NotNil(t, localNodeStatus)
			require.Equal(t, uint64(0), localNodeStatus.Gen[localNode])
			require.Equal(t, "", localNodeStatus.Monitor.Status)
		})
	})

	t.Run("ApplyFull vs CommitPending", func(t *testing.T) {
		status = bus.GetStatus()
		localNodeStatus := daemondata.GetNodeStatus(status, localNode)
		localNodeStatus.Monitor.Status = "CommitPending-1"
		localNodeStatus.Gen[localNode] = 1

		t.Logf("ApplyFull localnode updates CommitPending-1")
		bus.ApplyFull(localNode, localNodeStatus)

		full := LoadFull(t, "full-node2-t1.json")
		t.Logf("ApplyFull remote updates")
		bus.ApplyFull(remoteHost, full)

		status = bus.GetStatus()
		t.Run("pending are not applied until commit", func(t *testing.T) {
			status = bus.GetStatus()
			localNodeStatus = daemondata.GetNodeStatus(status, localNode)
			require.Equal(t, localNodeStatus.Monitor.Status, "")
			require.Equal(t, uint64(0), localNodeStatus.Gen[localNode])
			remoteNodeStatus = daemondata.GetNodeStatus(status, remoteHost)
			require.Nil(t, remoteNodeStatus)

			t.Log("CommitPending")
			bus.CommitPending()
			t.Run("result has pending changes applied after CommitPending", func(t *testing.T) {
				status = bus.GetStatus()
				localNodeStatus = daemondata.GetNodeStatus(status, localNode)
				require.Equal(t, localNodeStatus.Monitor.Status, "CommitPending-1")
				require.Equal(t, uint64(1), localNodeStatus.Gen[localNode])
				remoteNodeStatus = daemondata.GetNodeStatus(status, remoteHost)
				require.Equal(t, remoteNodeStatus.Monitor.Status, "idle-t1")
			})
			t.Run("localNode now know about remote gens", func(t *testing.T) {
				localNodeStatus = daemondata.GetNodeStatus(status, localNode)
				require.Len(t, localNodeStatus.Gen, 2)
				require.Equal(t, localNodeStatus.Gen[remoteHost], full.Gen[remoteHost])
			})
		})
	})

	t.Run("ApplyPatch vs CommitPending", func(t *testing.T) {
		full := LoadFull(t, "full-node2-t1.json")
		t.Logf("ApplyFull remote updates t1")
		bus.ApplyFull(remoteHost, full)
		patchMsg := LoadPatch(t, "patch-node2-t2.json")
		t.Logf("ApplyPatch remote updates")
		err = bus.ApplyPatch(remoteHost, patchMsg)
		require.Nil(t, err)
		t.Log("CommitPending")
		bus.CommitPending()
		status = bus.GetStatus()
		remoteNodeStatus = daemondata.GetNodeStatus(status, remoteHost)
		require.NotNil(t, remoteNodeStatus)
		t.Run("verify patch applied", func(t *testing.T) {
			require.Equal(t, remoteNodeStatus.Stats.Load15M, 0.5)
			require.Equal(t, remoteNodeStatus.Stats.MemTotalMB, uint64(1000))
			require.Equal(t, remoteNodeStatus.Stats.MemAvailPct, uint64(10))
			require.Equal(t, remoteNodeStatus.Stats.SwapTotalMB, uint64(11))
		})
		t.Run("expect local node know now new remote gen", func(t *testing.T) {
			localNodeStatus = daemondata.GetNodeStatus(status, localNode)
			require.Equal(t, uint64(21), localNodeStatus.Gen[remoteHost])
		})
		t.Run("verify older gen patch are not applied", func(t *testing.T) {
			require.Equal(t, remoteNodeStatus.Stats.Score, full.Stats.Score)
		})
		t.Run("When delta contains patch in future", func(t *testing.T) {
			patchMsg = LoadPatch(t, "patch-node2-t4.json")
			t.Logf("ApplyPatch remote updates")
			err = bus.ApplyPatch(remoteHost, patchMsg)
			t.Run("expect ApplyPatch error", func(t *testing.T) {
				require.Equal(t, "ApplyRemotePatch invalid patch gen: 23", err.Error())
			})
			t.Run("ensure future delta not applied to pending", func(t *testing.T) {
				t.Log("CommitPending")
				bus.CommitPending()
				status = bus.GetStatus()
				remoteNodeStatus = daemondata.GetNodeStatus(status, remoteHost)
				require.NotNil(t, remoteNodeStatus)
				require.Equal(t, uint64(21), remoteNodeStatus.Gen[remoteHost])
			})
			t.Run("expect local node now needs full from remote", func(t *testing.T) {
				localNodeStatus = daemondata.GetNodeStatus(status, localNode)
				require.Equal(t, uint64(0), localNodeStatus.Gen[remoteHost])
			})
		})
	})

	t.Logf("stats: %v", bus.Stats())

}
