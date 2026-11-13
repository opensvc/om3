package drbdmon

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/opensvc/om3/v3/daemon/evwatch"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The events2 lines captured on a node while a peer took its drbd resource
// down and up: the state changes name the resource, the promotion score,
// path and helper lines, and the initial state, do not.
func TestParseEvent(t *testing.T) {
	const res = "testdrbd.root.vol.reliable-leopard"
	states := newEventStates()
	for _, tc := range []struct {
		line string
		want bool
	}{
		{"exists resource name:" + res + " role:Secondary suspended:no force-io-failures:no may_promote:yes promotion_score:10103", false},
		{"exists -", false},
		{"change resource name:" + res + " may_promote:yes promotion_score:10102", false},
		{"change connection name:" + res + " peer-node-id:2 conn-name:dev2n3 connection:TearDown role:Unknown", true},
		{"change peer-device name:" + res + " peer-node-id:2 conn-name:dev2n3 volume:0 replication:Off peer-disk:DUnknown peer-client:no", true},
		{"change path name:" + res + " peer-node-id:2 conn-name:dev2n3 local:ipv4:10.29.0.11:7293 peer:ipv4:10.29.0.13:7296 established:no", false},
		{"call helper name:" + res + " peer-node-id:2 conn-name:dev2n3 helper:disconnected", false},
		{"response helper name:" + res + " peer-node-id:2 conn-name:dev2n3 helper:disconnected status:0", false},
		{"change connection name:" + res + " peer-node-id:2 conn-name:dev2n3 connection:Connected role:Secondary", true},
		{"change peer-device name:" + res + " peer-node-id:2 conn-name:dev2n3 volume:0 replication:Established peer-disk:UpToDate peer-client:no", true},
		{"create resource name:" + res + " role:Secondary suspended:no", true},
		{"destroy device name:" + res + " volume:0 minor:4", true},
		{"change device name:" + res + " volume:0 minor:4 disk:Inconsistent", true},
		{"", false},
	} {
		name, ok := states.parse(tc.line)
		require.Equal(t, tc.want, ok, tc.line)
		if ok {
			require.Equal(t, res, name, tc.line)
		}
	}
}

// A process opening an unused drbd device, as lvs scanning the block devices
// does, changes its open count, and events2 repeats every field of the device
// with it, its disk state too: that is no change of state, and nothing is
// notified, where a status refresh used to be every 5 seconds. A change of a
// state key still is, and the objects are told apart by peer and volume.
func TestParseEventOnlyCountsChangedValues(t *testing.T) {
	const res = "c33loop.root.vol.cluster33"
	states := newEventStates()
	for _, tc := range []struct {
		line string
		want bool
	}{
		{"exists resource name:" + res + " role:Primary suspended:no", false},
		{"exists device name:" + res + " volume:0 minor:0 backing_dev:/dev/data/c33loop disk:UpToDate client:no open:no quorum:yes", false},
		{"exists peer-device name:" + res + " peer-node-id:1 conn-name:n2 volume:0 replication:Established peer-disk:UpToDate peer-client:no", false},
		{"exists -", false},
		{"change device name:" + res + " volume:0 minor:0 backing_dev:/dev/data/c33loop disk:UpToDate client:no open:yes quorum:yes", false},
		{"change device name:" + res + " volume:0 minor:0 backing_dev:/dev/data/c33loop disk:UpToDate client:no open:no quorum:yes", false},
		{"change device name:" + res + " volume:0 minor:0 disk:UpToDate client:no open:no quorum:no", false},
		{"change peer-device name:" + res + " peer-node-id:1 conn-name:n2 volume:0 replication:Established peer-disk:UpToDate peer-client:no", false},
		{"change peer-device name:" + res + " peer-node-id:1 conn-name:n2 volume:0 replication:SyncSource peer-disk:Inconsistent peer-client:no", true},
		{"change peer-device name:" + res + " peer-node-id:2 conn-name:n3 volume:0 replication:SyncSource peer-disk:Inconsistent peer-client:no", true},
		{"change resource name:" + res + " role:Secondary suspended:no", true},
		{"change resource name:" + res + " role:Secondary suspended:no", false},
		{"destroy resource name:" + res, true},
		{"create device name:" + res + " volume:0 minor:0 disk:Diskless", true},
	} {
		_, ok := states.parse(tc.line)
		require.Equal(t, tc.want, ok, tc.line)
	}
}

// The probe accepts a drbdsetup printing the current state and exiting, and
// refuses one that does not know the event stream, with its message.
func TestProbe(t *testing.T) {
	d := t.TempDir()
	script := func(name, body string) string {
		p := filepath.Join(d, name)
		require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755))
		return p
	}
	src := &source{log: plog.NewDefaultLogger()}

	src.drbdsetup = script("supported", `echo "exists -"`)
	require.NoError(t, src.probe(context.Background()))

	src.drbdsetup = script("unsupported", `echo "drbdsetup: unknown command 'events2'" >&2; exit 20`)
	err := src.probe(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown command 'events2'")
}

// installDrbdsetup puts a drbdsetup script first in the PATH, and the drbd
// kernel module loaded or not.
// systemPath is the PATH the tests start with, which the scripts of a test
// run their commands from, whatever an earlier test case set.
var systemPath = os.Getenv("PATH")

func installDrbdsetup(t *testing.T, body string, moduleLoaded bool) {
	t.Helper()
	d := t.TempDir()
	if body != "" {
		require.NoError(t, os.WriteFile(filepath.Join(d, "drbdsetup"), []byte("#!/bin/sh\n"+body+"\n"), 0o755))
		// The script runs the commands of the node, the drbdsetup of the
		// test found first.
		t.Setenv("PATH", d+":"+systemPath)
	} else {
		t.Setenv("PATH", d)
	}
	former := procDRBD
	procDRBD = filepath.Join(d, "proc-drbd")
	t.Cleanup(func() { procDRBD = former })
	if moduleLoaded {
		require.NoError(t, os.WriteFile(procDRBD, nil, 0o644))
	}
}

// A node with no drbdsetup, or one whose drbd does not know the event
// stream, can not run the source; one whose kernel module is not loaded yet
// is checked by the stream once it is.
func TestSourceCheck(t *testing.T) {
	src := &source{log: plog.NewDefaultLogger()}
	installDrbdsetup(t, "", true)
	require.ErrorIs(t, src.Check(context.Background()), evwatch.ErrUnsupported)

	installDrbdsetup(t, `echo "drbdsetup: unknown command 'events2'" >&2; exit 20`, true)
	err := src.Check(context.Background())
	require.ErrorIs(t, err, evwatch.ErrUnsupported)
	require.Contains(t, err.Error(), "unknown command 'events2'")

	installDrbdsetup(t, `sleep 30`, true)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	err = src.Check(ctx)
	cancel()
	require.Error(t, err)
	require.NotErrorIs(t, err, evwatch.ErrUnsupported, "a probe timing out is tried again")

	installDrbdsetup(t, `exit 20`, false)
	require.NoError(t, src.Check(context.Background()), "the module not loaded, the probe waits for it")

	installDrbdsetup(t, `echo "exists -"`, true)
	require.NoError(t, src.Check(context.Background()))
}

// The stream is ready once its initial dump of the state ends, and emits
// the drbd resources whose state changes after.
func TestSourceRun(t *testing.T) {
	installDrbdsetup(t, `[ "$2" = "--now" ] && { echo "exists -"; exit 0; }
echo "exists resource name:r1 role:Secondary suspended:no"
echo "exists -"
echo "change resource name:r1 role:Primary suspended:no"
sleep 60`, true)
	src := &source{log: plog.NewDefaultLogger()}
	require.NoError(t, src.Check(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	var readies int
	var names []string
	done := make(chan error, 1)
	var mu sync.Mutex
	go func() {
		done <- src.Run(ctx, func() {
			mu.Lock()
			readies++
			mu.Unlock()
		}, func(name string) {
			mu.Lock()
			names = append(names, name)
			mu.Unlock()
		})
	}()
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return readies == 1 && len(names) == 1
	}, 5*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream did not stop with its context")
	}
	assert.Equal(t, []string{"r1"}, names)
}
