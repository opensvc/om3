package object_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"

	_ "github.com/opensvc/om3/v3/drivers/resappsimple"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	_ "github.com/opensvc/om3/v3/drivers/resfsdir"
)

func setupKeySources(t *testing.T) {
	t.Helper()
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
}

// keySourceAlerts returns the comments of the key source alerts of the
// validation of the configuration of a svc of the dev namespace.
func keySourceAlerts(t *testing.T, conf string) []string {
	t.Helper()
	p := naming.Path{Namespace: "dev", Kind: naming.KindSvc, Name: "web"}
	o, err := object.NewSvc(p, object.WithConfigData([]byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"+conf)), object.WithVolatile(true))
	require.NoError(t, err)
	alerts, err := o.Config().Validate()
	require.NoError(t, err)
	l := make([]string, 0)
	for _, a := range alerts {
		m := a.Unstructured()
		if m["kind"] != "key source error" {
			continue
		}
		assert.Equal(t, "error", m["level"])
		l = append(l, m["key"].(string)+": "+m["comment"].(string))
	}
	return l
}

// Two keywords seeding a key from two sources are refused, wherever they are
// written: in the section, in DEFAULT, or read by a reference, and whichever
// comes first. Two keywords seeding it from the same source are not.
func TestValidateKeySourceConflict(t *testing.T) {
	setupKeySources(t)
	head := t.TempDir()
	for name, tc := range map[string]struct {
		conf     string
		conflict bool
	}{
		"same source": {
			conf: "[app#1]\ntype = simple\nstart = /bin/true\nenvironment = PW from ./sec/db key pw source random\n" +
				"[app#2]\ntype = simple\nstart = /bin/true\nenvironment = DB_PW from ./sec/db key pw source random\n",
		},
		"in sections": {
			conf: "[app#1]\ntype = simple\nstart = /bin/true\nenvironment = PW from ./sec/db key pw source random\n" +
				"[app#2]\ntype = simple\nstart = /bin/true\nenvironment = DB_PW from ./sec/db key pw source https://example.com/pw\n",
			conflict: true,
		},
		"inherited from DEFAULT": {
			conf: "environment = PW from ./sec/db key pw source random\n" +
				"[container#1]\ntype = docker\nimage = alpine\n" +
				"[app#2]\ntype = simple\nstart = /bin/true\nenvironment = DB_PW from ./sec/db key pw source https://example.com/pw\n",
			conflict: true,
		},
		"read by reference, install first": {
			conf: "[env]\npw = PW from ./sec/db key pw source random\n" +
				"[fs#1]\ntype = directory\npath = " + head + "\ninstall = /pw from ./sec/db key pw source /etc/hostname\n" +
				"[app#1]\ntype = simple\nstart = /bin/true\nenvironment = {env.pw}\n",
			conflict: true,
		},
		"read by reference, environment first": {
			conf: "[env]\npw = PW from ./sec/db key pw source random\n" +
				"[app#1]\ntype = simple\nstart = /bin/true\nenvironment = {env.pw}\n" +
				"[fs#1]\ntype = directory\npath = " + head + "\ninstall = /pw from ./sec/db key pw source /etc/hostname\n",
			conflict: true,
		},
		"no source on one": {
			conf: "[app#1]\ntype = simple\nstart = /bin/true\nenvironment = PW from ./sec/db key pw source random\n" +
				"[app#2]\ntype = simple\nstart = /bin/true\nenvironment = DB_PW from ./sec/db key pw\n",
		},
	} {
		l := keySourceAlerts(t, tc.conf)
		if !tc.conflict {
			assert.Empty(t, l, name)
			continue
		}
		require.Len(t, l, 1, name)
		assert.Contains(t, l[0], "dev/sec/db key pw is seeded from source", name)
		assert.Contains(t, l[0], "a key has one source", name)
	}
}

// A key of a datastore of another namespace is not seeded, whatever the
// source: the datastore sharing its keys lets the object read them.
func TestValidateKeySourceOtherNamespace(t *testing.T) {
	setupKeySources(t)
	head := t.TempDir()
	l := keySourceAlerts(t, "[app#1]\ntype = simple\nstart = /bin/true\nenvironment = PW from other/sec/db key pw source random\n"+
		"[fs#1]\ntype = directory\npath = "+head+"\ninstall = /x from other/cfg/c key x source https://example.com/x\n    /y from other/cfg/c key y\n")
	require.Len(t, l, 2)
	assert.Contains(t, strings.Join(l, "\n"), "app#1.environment: other/sec/db key pw source random: a key of a datastore of another namespace is not seeded")
	assert.Contains(t, strings.Join(l, "\n"), "fs#1.install: other/cfg/c key x source https://example.com/x")
}

// A source on an install line seeding no key named is refused by the
// validation: a key pattern, or a key generated in the datastores of a
// pattern.
func TestValidateKeySourcePattern(t *testing.T) {
	setupKeySources(t)
	head := t.TempDir()
	l := keySourceAlerts(t, "[fs#1]\ntype = directory\npath = "+head+"\ninstall = /conf/ from ./cfg/c key conf/* source https://example.com/c\n    /certs/*.pem from ./sec/haproxy-* key fullpem source random\n")
	require.Len(t, l, 1, "one keyword, its refusals joined")
	assert.Contains(t, l[0], "not the keys of a pattern")
	assert.Contains(t, l[0], "add it with each datastore")
}

// newKeySourcesSvc writes the configuration of a svc of the dev namespace,
// and returns the svc.
func newKeySourcesSvc(t *testing.T, conf string) object.Svc {
	t.Helper()
	p := naming.Path{Namespace: "dev", Kind: naming.KindSvc, Name: "web"}
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0700))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"+conf), 0600))
	o, err := object.NewSvc(p)
	require.NoError(t, err)
	return o
}

func decodeKey(t *testing.T, p naming.Path, k string) string {
	t.Helper()
	ds, err := object.NewDataStore(p, object.WithVolatile(true))
	require.NoError(t, err)
	b, err := ds.DecodeKey(k)
	require.NoError(t, err)
	return string(b)
}

// The provision of the leader creates the datastores of the namespace and the
// keys missing: a key generated, a key read from its source. A key there is
// never written again, and a start generates none.
func TestSeedKeySources(t *testing.T) {
	setupKeySources(t)
	src := filepath.Join(t.TempDir(), "u.conf")
	require.NoError(t, os.WriteFile(src, []byte("from-file"), 0600))
	head := t.TempDir()
	sec := naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "db"}
	cfg := naming.Path{Namespace: "dev", Kind: naming.KindCfg, Name: "c"}
	o := newKeySourcesSvc(t, "environment = PW from ./sec/db key pw source random\n"+
		"[app#1]\ntype = simple\nstart = /bin/true\n"+
		"[app#2]\ntype = simple\nstart = /bin/true\nenvironment = U from ./cfg/c key u source "+src+"\n"+
		"[app#3]\ntype = simple\nstart = /bin/true\ndisable = true\nenvironment = D from ./cfg/c key d source "+src+"\n"+
		"[fs#1]\ntype = directory\npath = "+head+"\ninstall = /pw2 from ./sec/db key pw2 source random\n")

	// A start generates no key, and reads the other sources.
	require.NoError(t, object.SeedKeySources(context.Background(), o, false))
	assert.False(t, object.HasDataStoreKey(sec, "pw"))
	assert.Equal(t, "from-file", decodeKey(t, cfg, "u"))
	assert.False(t, object.HasDataStoreKey(cfg, "d"), "a disabled resource seeds nothing")

	ctx := actioncontext.WithLeader(context.Background(), true)
	require.NoError(t, object.SeedKeySources(ctx, o, true))
	pw := decodeKey(t, sec, "pw")
	pw2 := decodeKey(t, sec, "pw2")
	assert.Len(t, pw, 32)
	assert.Len(t, pw2, 32)
	assert.NotEqual(t, pw, pw2)

	// The keys there are left as they are.
	require.NoError(t, os.WriteFile(src, []byte("changed"), 0600))
	require.NoError(t, object.SeedKeySources(ctx, o, true))
	assert.Equal(t, pw, decodeKey(t, sec, "pw"))
	assert.Equal(t, pw2, decodeKey(t, sec, "pw2"))
	assert.Equal(t, "from-file", decodeKey(t, cfg, "u"))
}

// The provision of a node other than the leader generates no key: it waits
// for the one the leader generates, and fails naming it when it does not
// come.
func TestSeedKeySourcesNotLeader(t *testing.T) {
	setupKeySources(t)
	defer object.SetRandomKeyWait(300*time.Millisecond, 50*time.Millisecond)()
	sec := naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "db"}
	o := newKeySourcesSvc(t, "[app#1]\ntype = simple\nstart = /bin/true\nenvironment = PW from ./sec/db key pw source random\n")

	err := object.SeedKeySources(context.Background(), o, true)
	var e object.ErrRandomKeyMissing
	require.ErrorAs(t, err, &e)
	assert.Equal(t, sec, e.Path)
	assert.Equal(t, "pw", e.Key)
	assert.Contains(t, err.Error(), "app#1.environment")
	assert.False(t, sec.Exists(), "the datastore is the leader's to create")

	// The key the leader generated reaches the node while it waits.
	done := make(chan error)
	go func() {
		done <- object.SeedKeySources(context.Background(), o, true)
	}()
	time.Sleep(100 * time.Millisecond)
	ds, err := object.NewSec(sec)
	require.NoError(t, err)
	require.NoError(t, ds.AddKey("pw", []byte("from-leader")))
	require.NoError(t, <-done)
	assert.Equal(t, "from-leader", decodeKey(t, sec, "pw"))
}

// A key of a datastore of another namespace is never seeded, the datastore
// sharing its keys with the object or not.
func TestSeedKeyOtherNamespace(t *testing.T) {
	setupKeySources(t)
	other := naming.Path{Namespace: "other", Kind: naming.KindSec, Name: "db"}
	require.NoError(t, os.MkdirAll(filepath.Dir(other.ConfigFile()), 0700))
	require.NoError(t, os.WriteFile(other.ConfigFile(), []byte("[DEFAULT]\nshare = dev\n"), 0600))
	ds, err := object.NewDataStore(other)
	require.NoError(t, err)
	require.True(t, ds.Allow("dev"))

	p := naming.Path{Namespace: "dev", Kind: naming.KindSvc, Name: "web"}
	err = object.SeedKey(context.Background(), ds, "pw", keywords.SourceRandom, plog.NewDefaultLogger(), p, "")
	require.ErrorIs(t, err, object.ErrSeedOtherNamespace)
	assert.False(t, object.HasDataStoreKey(other, "pw"))

	ctx := actioncontext.WithLeader(context.Background(), true)
	o := newKeySourcesSvc(t, "[app#1]\ntype = simple\nstart = /bin/true\nenvironment = PW from other/sec/db key pw source random\n")
	err = object.SeedKeySources(ctx, o, true)
	require.ErrorIs(t, err, object.ErrSeedOtherNamespace)
	assert.False(t, object.HasDataStoreKey(other, "pw"))
}

// A source that is no regular file is refused, and never read: a device as
// /dev/urandom would never end, a fifo would block until a writer comes.
func TestSeedKeyNotRegular(t *testing.T) {
	setupKeySources(t)
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	require.NoError(t, syscall.Mkfifo(fifo, 0600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink("/dev/urandom", link))

	sec := naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "db"}
	ds, _, err := object.EnsureDataStore(sec, "dev")
	require.NoError(t, err)
	p := naming.Path{Namespace: "dev", Kind: naming.KindSvc, Name: "web"}
	for _, source := range []string{"/dev/urandom", "/dev/null", fifo, link, dir} {
		done := make(chan error, 1)
		go func() { done <- object.SeedKey(context.Background(), ds, "k", source, plog.NewDefaultLogger(), p, "") }()
		select {
		case err := <-done:
			require.ErrorIs(t, err, object.ErrSourceNotRegular, source)
			assert.Contains(t, err.Error(), "source random", source)
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the seeding hangs", source)
		}
	}
	assert.False(t, object.HasDataStoreKey(sec, "k"))
}

// An install line seeding a key generated fails when the key is missing,
// at its start as at any install, rather than generating one: the provision
// of the leader generates it.
func TestInstallRandomKeyMissing(t *testing.T) {
	setupKeySources(t)
	head := t.TempDir()
	sec := naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "db"}
	o := newKeySourcesSvc(t, "[fs#1]\ntype = directory\npath = "+head+"\ninstall = /pw from ./sec/db key pw source random\n")
	_ = o.Resources()
	r := o.ResourceByID("fs#1")
	require.NotNil(t, r)
	installer, ok := r.(interface{ Do(context.Context) error })
	require.True(t, ok)

	err := installer.Do(context.Background())
	var e object.ErrRandomKeyMissing
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "pw", e.Key)
	assert.False(t, sec.Exists(), "the install creates no datastore for a key generated")

	ctx := actioncontext.WithLeader(context.Background(), true)
	require.NoError(t, object.SeedKeySources(ctx, o, true))
	require.NoError(t, installer.Do(context.Background()))
	b, err := os.ReadFile(filepath.Join(head, "pw"))
	require.NoError(t, err)
	assert.Equal(t, decodeKey(t, sec, "pw"), string(b))
}
