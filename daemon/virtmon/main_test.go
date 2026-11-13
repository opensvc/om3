package virtmon

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/evwatch"
	"github.com/opensvc/om3/v3/util/plog"
)

// The lines virsh 10.0.0 prints, captured on a dev cluster node for a domain
// created, suspended, resumed and destroyed, with and without --timestamp.
func TestParseEvent(t *testing.T) {
	for _, tc := range []struct {
		line   string
		ok     bool
		domain string
		event  string
		detail string
	}{
		{"event 'lifecycle' for domain 'omtest-evt': Resumed Unpaused", true, "omtest-evt", "Resumed", "Unpaused"},
		{"event 'lifecycle' for domain 'omtest-evt': Started Booted", true, "omtest-evt", "Started", "Booted"},
		{"event 'lifecycle' for domain 'omtest-evt': Suspended Paused", true, "omtest-evt", "Suspended", "Paused"},
		{"event 'lifecycle' for domain 'omtest-evt': Shutdown Finished after host request", true, "omtest-evt", "Shutdown", "Finished after host request"},
		{"event 'lifecycle' for domain 'omtest-evt': Stopped Destroyed", true, "omtest-evt", "Stopped", "Destroyed"},
		{"2026-11-13 17:06:57.884+0000: event 'lifecycle' for domain 'omtest-evt': Resumed Unpaused", true, "omtest-evt", "Resumed", "Unpaused"},
		{"event 'lifecycle' for domain 'ns1..svc1.container.1': Stopped Failed", true, "ns1..svc1.container.1", "Stopped", "Failed"},
		{"event 'lifecycle' for domain vm1: Started Booted", true, "vm1", "Started", "Booted"},
		{"event 'lifecycle' for domain 'a vm: odd': Stopped Destroyed", true, "a vm: odd", "Stopped", "Destroyed"},
		{"events received: 4", false, "", "", ""},
		{"error: Disconnected from qemu:///system due to end of file", false, "", "", ""},
		{"", false, "", "", ""},
		{"event 'lifecycle' for domain '': Started Booted", false, "", "", ""},
	} {
		ev, ok := parseEvent(tc.line)
		require.Equal(t, tc.ok, ok, tc.line)
		if !ok {
			continue
		}
		assert.Equal(t, tc.domain, ev.Domain, tc.line)
		assert.Equal(t, tc.event, ev.Event, tc.line)
		assert.Equal(t, tc.detail, ev.Detail, tc.line)
	}
}

// installVirsh puts a virsh script first in the PATH, or none at all.
func installVirsh(t *testing.T, body string) {
	t.Helper()
	d := t.TempDir()
	if body == "" {
		t.Setenv("PATH", d)
		return
	}
	require.NoError(t, os.WriteFile(filepath.Join(d, "virsh"), []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	t.Setenv("PATH", d+":"+os.Getenv("PATH"))
}

// A node with no virsh, or one too old to print the events, can not run the
// source.
func TestSourceCheck(t *testing.T) {
	src := &source{log: plog.NewDefaultLogger()}
	installVirsh(t, "")
	require.ErrorIs(t, src.Check(context.Background()), evwatch.ErrUnsupported)

	installVirsh(t, `echo "error: unknown command: 'event'" >&2; exit 1`)
	require.ErrorIs(t, src.Check(context.Background()), evwatch.ErrUnsupported)

	installVirsh(t, `echo "  --domain <string>  filter by domain name"`)
	require.ErrorIs(t, src.Check(context.Background()), evwatch.ErrUnsupported, "no --loop")

	installVirsh(t, `echo "  --loop  loop until timeout or interrupt"`)
	require.NoError(t, src.Check(context.Background()))
}

// The stream is read on a read-only connection, untranslated, ready once it
// ran a while, and emits the domains its events name.
func TestSourceRun(t *testing.T) {
	installVirsh(t, `[ "$1" = "help" ] && { echo "--loop"; exit 0; }
[ "$1 $2 $3 $4 $5" = "-r event --event lifecycle --loop" ] || exit 2
[ "$LC_ALL" = "C" ] || exit 3
echo "event 'lifecycle' for domain 'vm1': Started Booted"
echo "events received: 1"
sleep 60`)
	src := &source{log: plog.NewDefaultLogger()}
	require.NoError(t, src.Check(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var readies int
	var names []string
	done := make(chan error, 1)
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
	assert.Equal(t, []string{"vm1"}, names)
}

// A stream ending before it ran a while is never taken as ready, and says
// why it ended.
func TestSourceRunEndingEarly(t *testing.T) {
	installVirsh(t, `[ "$1" = "help" ] && { echo "--loop"; exit 0; }
echo "error: failed to connect to the hypervisor" >&2
exit 1`)
	src := &source{log: plog.NewDefaultLogger()}
	require.NoError(t, src.Check(context.Background()))
	readies := 0
	err := src.Run(context.Background(), func() { readies++ }, func(string) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to the hypervisor")
	time.Sleep(readyDelay + 200*time.Millisecond)
	assert.Zero(t, readies)
}
