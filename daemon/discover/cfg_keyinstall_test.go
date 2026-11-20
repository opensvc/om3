package discover

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/priority"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
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

func withTypedResources(rs map[string]resource.Status) *instance.Status {
	return &instance.Status{Resources: instance.ResourceStatuses(rs)}
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
		{"a directory, whose status does not apply", withTypedResources(map[string]resource.Status{"fs#1": {Type: "fs.directory", Status: status.NotApplicable}}), uses("testigw/sec/web-tls"), mtime, noTemplate, true},
		{"a filesystem whose status does not apply, as with no device", withTypedResources(map[string]resource.Status{"fs#1": {Type: "fs.host", Status: status.NotApplicable}}), uses("testigw/sec/web-tls"), mtime, noTemplate, false},
		{"a volume whose status does not apply, as failing to configure", withTypedResources(map[string]resource.Status{"volume#1": {Type: "volume", Status: status.NotApplicable}}), uses("testigw/sec/web-tls"), mtime, noTemplate, false},
		{"a directory missing", withResources(map[string]status.T{"fs#1": status.Down}), uses("testigw/sec/web-tls"), mtime, noTemplate, false},
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

// A new configuration of a local svc instance whose resource starts to use a
// datastore, as a resource enabled again or given an install line, refreshes
// its keys, even when another resource of the object used it already. The
// first configuration seen only records what the resources use, and a
// datastore a resource stops using, a vol, or a datastore that does not
// exist here, refreshes nothing.
func TestOnLocalConfigUpdatedRefreshesTheDatastoresNewlyUsed(t *testing.T) {
	testhelper.Setup(t)
	for _, s := range []string{"ns1/cfg/a", "ns1/cfg/b", "ns1/sec/web-a", "ns1/sec/web-b", "ns1/cfg/web-c"} {
		p, err := naming.ParsePath(s)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
		require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\n"), 0o600))
	}

	var (
		mu        sync.Mutex
		refreshed []string
	)
	ki := newKeyInstaller(context.Background(), func() int { return 4 }, func(_ context.Context, p naming.Path, starting func()) {
		starting()
		mu.Lock()
		refreshed = append(refreshed, p.String())
		mu.Unlock()
	})
	m := &Manager{
		localhost:    "n1",
		log:          plog.NewDefaultLogger(),
		keyInstaller: ki,
		localUses:    make(map[naming.Path]map[string][]string),
		cfgDeleting:  make(map[naming.Path]bool),
	}
	svc := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "web"}

	// update publishes a configuration of p whose resources use what uses
	// says, by rid, and returns the datastores refreshed.
	update := func(p naming.Path, uses map[string][]string) []string {
		t.Helper()
		mu.Lock()
		refreshed = nil
		mu.Unlock()
		resources := make(instance.ResourceConfigs)
		var all []string
		for rid, l := range uses {
			resources[rid] = instance.ResourceConfig{Uses: l}
			all = append(all, l...)
		}
		m.onInstanceConfigUpdated(&msgbus.InstanceConfigUpdated{Path: p, Node: "n1", Value: instance.Config{
			ActorConfig: &instance.ActorConfig{Resources: resources, Uses: all},
		}})
		require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		l := append([]string{}, refreshed...)
		slices.Sort(l)
		return l
	}
	type uses = map[string][]string

	assert.Empty(t, update(svc, uses{"fs#1": {"ns1/cfg/a"}}), "the first configuration seen records what it uses")
	assert.Empty(t, update(svc, uses{"fs#1": {"ns1/cfg/a"}}), "nothing newly used")
	assert.Equal(t, []string{"ns1/cfg/a"}, update(svc, uses{"fs#1": {"ns1/cfg/a"}, "fs#2": {"ns1/cfg/a"}}),
		"a resource enabled again using a datastore another resource uses already")
	assert.Equal(t, []string{"ns1/cfg/b"}, update(svc, uses{"fs#1": {"ns1/cfg/a", "ns1/cfg/b", "ns1/cfg/missing", "ns1/vol/data"}, "fs#2": {"ns1/cfg/a"}}),
		"a datastore newly used, not one missing here nor a vol")
	assert.Empty(t, update(svc, uses{"fs#1": {"ns1/cfg/b"}}), "datastores no longer used, a resource disabled")
	assert.Equal(t, []string{"ns1/sec/web-a", "ns1/sec/web-b"}, update(svc, uses{"fs#1": {"ns1/cfg/b"}, "volume#1": {"ns1/sec/web-*"}}),
		"the datastores a pattern newly used matches")

	m.onInstanceConfigDeleting(&msgbus.InstanceConfigDeleting{Path: svc, Node: "n1"})
	assert.Empty(t, update(svc, uses{"fs#1": {"ns1/cfg/a", "ns1/cfg/b"}}), "a configuration seen first again only records")

	vol := naming.Path{Namespace: "ns1", Kind: naming.KindVol, Name: "data"}
	update(vol, uses{"fs#1": {"ns1/cfg/a"}})
	assert.Empty(t, update(vol, uses{"fs#1": {"ns1/cfg/a", "ns1/cfg/b"}}), "a vol installs no key")
}

// The datastores a pattern names are the local ones it matches, whatever
// object filter the environment of the daemon holds.
func TestLocalDataStoresOfAPattern(t *testing.T) {
	testhelper.Setup(t)
	for _, s := range []string{"ns1/sec/web-a", "ns1/sec/web-b", "ns1/cfg/web-c", "ns1/svc/web-d", "ns2/sec/web-e"} {
		p, err := naming.ParsePath(s)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
		require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\n"), 0o600))
	}
	names := func(l []naming.Path) []string {
		var s []string
		for _, p := range l {
			s = append(s, p.String())
		}
		slices.Sort(s)
		return s
	}
	assert.Equal(t, []string{"ns1/sec/web-a", "ns1/sec/web-b"}, names(localDataStoresOf("ns1/sec/web-*")))
	assert.Empty(t, localDataStoresOf("ns1/*/web-*"), "a pattern names objects of one kind, as an install line does")
	assert.Empty(t, localDataStoresOf("ns1/svc/web-*"), "a svc is no datastore")

	t.Setenv("OSVC_NAMESPACE", "other")
	t.Setenv("OSVC_KIND", "svc")
	assert.Equal(t, []string{"ns1/sec/web-a", "ns1/sec/web-b"}, names(localDataStoresOf("ns1/sec/web-*")),
		"the object filter of the environment is not applied")
}
