package istat

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	nopPublisher struct{}

	logEntry struct {
		Message string `json:"message"`
		RID     string `json:"rid"`
	}
)

func (nopPublisher) Pub(pubsub.Messager, ...pubsub.Label) {}

// newTestT returns a T logging its json records to buf.
func newTestT(buf *bytes.Buffer) *T {
	t := New(nil)
	t.log = plog.NewLogger(zerolog.New(buf))
	t.publisher = nopPublisher{}
	return t
}

func logEntries(tt *testing.T, buf *bytes.Buffer) []logEntry {
	tt.Helper()
	var entries []logEntry
	d := json.NewDecoder(buf)
	for d.More() {
		var e logEntry
		require.NoError(tt, d.Decode(&e))
		entries = append(entries, e)
	}
	return entries
}

func TestOnInstanceStatusPostLogsResourceStatusChanges(tt *testing.T) {
	p, err := naming.ParsePath("svc1")
	require.NoError(tt, err)

	var buf bytes.Buffer
	t := newTestT(&buf)
	instance.ConfigData.Set(p, t.localhost, &instance.Config{})
	defer instance.ConfigData.Unset(p, t.localhost)

	now := time.Now()
	t.iStatusM[p.String()] = instance.Status{
		Avail:     status.Up,
		Overall:   status.Up,
		UpdatedAt: now,
		Resources: instance.ResourceStatuses{
			"app#1": resource.Status{Status: status.Up},
			"fs#1":  resource.Status{Status: status.Up},
			"ip#1":  resource.Status{Status: status.Up},
			"old#1": resource.Status{Status: status.Up},
		},
	}
	t.onInstanceStatusPost(&msgbus.InstanceStatusPost{
		Path: p,
		Node: t.localhost,
		Value: instance.Status{
			Avail:     status.Down,
			Overall:   status.Down,
			UpdatedAt: now.Add(time.Second),
			Resources: instance.ResourceStatuses{
				"app#1": resource.Status{Status: status.Down},
				"fs#1":  resource.Status{Status: status.Up},
				"ip#1":  resource.Status{Status: status.Down},
				"new#1": resource.Status{Status: status.Up},
			},
		},
	})

	require.Equal(tt, []logEntry{
		{Message: "svc1: change resource app#1 status up -> down", RID: "app#1"},
		{Message: "svc1: change resource ip#1 status up -> down", RID: "ip#1"},
		{Message: "svc1: change resource new#1 status undef -> up", RID: "new#1"},
		{Message: "svc1: change resource old#1 status up -> undef", RID: "old#1"},
		{Message: "svc1: change avail up -> down"},
		{Message: "svc1: change overall up -> down"},
	}, logEntries(tt, &buf))
}

func TestOnInstanceStatusPostLogsNoUnchangedResourceStatus(tt *testing.T) {
	p, err := naming.ParsePath("svc1")
	require.NoError(tt, err)

	var buf bytes.Buffer
	t := newTestT(&buf)
	instance.ConfigData.Set(p, t.localhost, &instance.Config{})
	defer instance.ConfigData.Unset(p, t.localhost)

	now := time.Now()
	value := instance.Status{
		Avail:     status.Up,
		Overall:   status.Up,
		UpdatedAt: now,
		Resources: instance.ResourceStatuses{
			"app#1": resource.Status{Status: status.Up},
		},
	}
	t.iStatusM[p.String()] = value
	value.UpdatedAt = now.Add(time.Second)
	t.onInstanceStatusPost(&msgbus.InstanceStatusPost{Path: p, Node: t.localhost, Value: value})

	require.Empty(tt, logEntries(tt, &buf))
}

// A run file names the session of a running action and is named after its
// pid. One that is no regular file, a symlink planted in the run directory or
// a fifo, is not read, and what is wrong with a run file is said.
func TestLoadRunningInfoReadsARegularRunFileOnly(t *testing.T) {
	dir := t.TempDir()
	sid := uuid.New()
	good := filepath.Join(dir, "123")
	require.NoError(t, os.WriteFile(good, []byte(sid.String()), 0600))
	info, err := LoadRunningInfo("task#1", good)
	require.NoError(t, err)
	assert.Equal(t, sid, info.SessionID)
	assert.Equal(t, 123, info.PID)

	link := filepath.Join(dir, "124")
	require.NoError(t, os.Symlink(good, link))
	_, err = LoadRunningInfo("task#1", link)
	assert.Error(t, err)

	fifo := filepath.Join(dir, "125")
	require.NoError(t, syscall.Mkfifo(fifo, 0600))
	_, err = LoadRunningInfo("task#1", fifo)
	assert.Error(t, err)

	bad := filepath.Join(dir, "notapid")
	require.NoError(t, os.WriteFile(bad, []byte("not a session"), 0600))
	_, err = LoadRunningInfo("task#1", bad)
	assert.Error(t, err)
}
