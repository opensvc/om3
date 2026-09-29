package drbdmon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The events2 lines captured on a node while a peer took its drbd resource
// down and up: the state changes name the resource, the promotion score,
// path and helper lines, and the initial state, do not.
func TestParseEvent(t *testing.T) {
	const res = "testdrbd.root.vol.reliable-leopard"
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
		name, ok := parseEvent(tc.line)
		require.Equal(t, tc.want, ok, tc.line)
		if ok {
			require.Equal(t, res, name, tc.line)
		}
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
	m := NewManager(0, nil)
	m.ctx = context.Background()

	m.drbdsetup = script("supported", `echo "exists -"`)
	require.NoError(t, m.probe())

	m.drbdsetup = script("unsupported", `echo "drbdsetup: unknown command 'events2'" >&2; exit 20`)
	err := m.probe()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown command 'events2'")
}
