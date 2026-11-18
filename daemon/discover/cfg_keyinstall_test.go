package discover

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/priority"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"
)

func withResources(rs map[string]status.T) *instance.Status {
	st := &instance.Status{Resources: instance.ResourceStatuses{}}
	for rid, s := range rs {
		st.Resources[rid] = resource.Status{Status: s}
	}
	return st
}

// A datastore fetched is refreshed in the local volumes when a local svc
// instance with a volume or fs resource up uses it, as its configuration
// says, and when its configuration can't tell: the install command is not
// run for nothing, and is run when in doubt.
func TestMayInstallKeysOf(t *testing.T) {
	store := naming.Path{Namespace: "testigw", Kind: naming.KindSec, Name: "web-tls"}
	mtime := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	upVolume := withResources(map[string]status.T{"volume#1": status.Up, "container#1": status.Up})
	uses := func(l ...string) *instance.Config {
		return &instance.Config{UpdatedAt: mtime, ActorConfig: &instance.ActorConfig{Uses: l}}
	}
	noTemplate := func(string) bool { return false }

	for _, tc := range []struct {
		name        string
		st          *instance.Status
		cfg         *instance.Config
		configMtime time.Time
		templates   func(string) bool
		want        bool
	}{
		{"uses the datastore", upVolume, uses("testigw/sec/web-tls"), mtime, noTemplate, true},
		{"uses a pattern matching the datastore", upVolume, uses("testigw/sec/web-*"), mtime, noTemplate, true},
		{"uses other datastores", upVolume, uses("testigw/sec/db", "testigw/vol/data"), mtime, noTemplate, false},
		{"uses nothing", upVolume, uses(), mtime, noTemplate, false},
		{"an fs up", withResources(map[string]status.T{"fs#1": status.Up}), uses("testigw/sec/web-tls"), mtime, noTemplate, true},
		{"a volume standby up", withResources(map[string]status.T{"volume#1": status.StandbyUp}), uses("testigw/sec/web-tls"), mtime, noTemplate, true},
		{"a volume down", withResources(map[string]status.T{"volume#1": status.Down}), uses("testigw/sec/web-tls"), mtime, noTemplate, false},
		{"no volume nor fs", withResources(map[string]status.T{"container#1": status.Up, "ip#1": status.Up}), uses("testigw/sec/web-tls"), mtime, noTemplate, false},
		{"no status", nil, uses("testigw/sec/web-tls"), mtime, noTemplate, false},
		{"no configuration known yet", upVolume, nil, mtime, noTemplate, true},
		{"no actor configuration known yet", upVolume, &instance.Config{UpdatedAt: mtime}, mtime, noTemplate, true},
		{"a configuration older than its file", upVolume, uses(), mtime.Add(time.Second), noTemplate, true},
		{"a template reading the datastore", upVolume, uses(), mtime, func(rid string) bool { return rid == "volume#1" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mayInstallKeysOf(store, tc.st, tc.cfg, func() time.Time { return tc.configMtime }, tc.templates)
			assert.Equal(t, tc.want, got)
		})
	}
}

// The refresh runs at the priority of the most urgent local instance using
// the datastore, an instance of another namespace counting when the
// datastore is shared with it, and is not run when none uses it.
func TestKeyRefreshPriority(t *testing.T) {
	testhelper.Setup(t)
	const localhost = "n1"
	store := naming.Path{Namespace: "ns1", Kind: naming.KindCfg, Name: "tls"}
	require.NoError(t, os.MkdirAll(filepath.Dir(store.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(store.ConfigFile(), []byte("[DEFAULT]\nshare = ns2\n"), 0o600))

	upVolume := withResources(map[string]status.T{"volume#1": status.Up})
	var paths []naming.Path
	add := func(p naming.Path, prio priority.T, uses ...string) {
		paths = append(paths, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
		require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\n"), 0o600))
		info, err := os.Stat(p.ConfigFile())
		require.NoError(t, err)
		instance.StatusData.Set(p, localhost, upVolume)
		instance.ConfigData.Set(p, localhost, &instance.Config{
			Priority:    prio,
			UpdatedAt:   info.ModTime(),
			ActorConfig: &instance.ActorConfig{Uses: uses},
		})
	}
	t.Cleanup(func() {
		for _, p := range paths {
			instance.StatusData.Unset(p, localhost)
			instance.ConfigData.Unset(p, localhost)
		}
	})
	m := &Manager{localhost: localhost, log: plog.NewDefaultLogger()}

	_, ok := m.keyRefreshPriority(store)
	assert.False(t, ok, "no local instance")

	add(naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "other"}, 1, "ns1/cfg/db")
	_, ok = m.keyRefreshPriority(store)
	assert.False(t, ok, "no local instance uses the datastore")

	add(naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "web"}, 60, "ns1/cfg/tls")
	prio, ok := m.keyRefreshPriority(store)
	assert.True(t, ok)
	assert.Equal(t, priority.T(60), prio)

	add(naming.Path{Namespace: "ns3", Kind: naming.KindSvc, Name: "web"}, 10, "ns1/cfg/tls")
	prio, ok = m.keyRefreshPriority(store)
	assert.True(t, ok)
	assert.Equal(t, priority.T(60), prio, "the datastore is not shared with ns3")

	add(naming.Path{Namespace: "ns2", Kind: naming.KindSvc, Name: "web"}, 20, "ns1/cfg/tls")
	prio, ok = m.keyRefreshPriority(store)
	assert.True(t, ok)
	assert.Equal(t, priority.T(20), prio, "the datastore is shared with ns2")
}
