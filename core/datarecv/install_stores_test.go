package datarecv

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

// A line naming its datastores by a pattern installs the key of each one,
// named after the part of the datastore name the * matched, follows the
// datastores created and deleted, and leaves the datastores the pattern does
// not name.
func TestALineOfADatastorePatternInstallsAFileEach(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "gw", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)
	store := func(name, value string) object.DataStore {
		ds, err := object.NewDataStore(naming.Path{Name: name, Kind: naming.KindCfg, Namespace: naming.NsRoot})
		require.NoError(t, err)
		require.NoError(t, ds.AddKey("fullpem", []byte(value)))
		return ds
	}
	store("haproxy-www.example.com", "www")
	api := store("haproxy-api.example.com", "api")
	store("haproxy", "gw")
	store("other-x", "x")

	head := t.TempDir()
	recv := &DataRecv{
		to:      headReceiver{o: o, head: head},
		Install: []string{"/certs/*.pem", "from", "./cfg/haproxy-*", "key", "fullpem"},
	}
	_, err = recv.install(context.Background())
	require.NoError(t, err)
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(head, "certs", name))
		if err != nil {
			return ""
		}
		return string(b)
	}
	assert.Equal(t, "www", read("www.example.com.pem"))
	assert.Equal(t, "api", read("api.example.com.pem"))
	entries, err := os.ReadDir(filepath.Join(head, "certs"))
	require.NoError(t, err)
	assert.Len(t, entries, 2, "the datastores the pattern does not name install nothing")

	created := store("haproxy-new.example.com", "new")
	assert.True(t, recv.HasMetadata(created.Path(), ""))
	_, err = recv.InstallFromDatastore(context.Background(), created)
	require.NoError(t, err)
	assert.Equal(t, "new", read("new.example.com.pem"), "a datastore created")

	require.NoError(t, os.Remove(api.Path().ConfigFile()))
	assert.True(t, recv.HasMetadata(api.Path(), ""), "a datastore deleted has its files removed")
	changed, err := recv.InstallFromDatastore(context.Background(), api)
	require.NoError(t, err)
	assert.True(t, changed, "the removal is a change of the line")
	assert.NoFileExists(t, filepath.Join(head, "certs", "api.example.com.pem"))
	assert.Equal(t, "www", read("www.example.com.pem"))
	v, err := ReadInstalled(filepath.Join(InstalledDir(p), "volume#1"))
	require.NoError(t, err)
	assert.Contains(t, v.Files, "/certs/api.example.com.pem", "the removal is reported")
}

// A required line naming its datastores by a pattern fails when none matches.
func TestARequiredDatastorePatternMatchingNoneFails(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "gw", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)

	recv := &DataRecv{
		to:      headReceiver{o: o, head: t.TempDir()},
		Install: []string{"/certs/*.pem", "from", "./cfg/haproxy-*", "key", "fullpem"},
	}
	_, err = recv.install(context.Background())
	assert.NoError(t, err, "a pattern matching none installs nothing")

	recv.Install = append(recv.Install, "required")
	_, err = recv.install(context.Background())
	assert.ErrorContains(t, err, "no datastore matches cfg/haproxy-*")
}

func TestPatternLineOfAnItem(t *testing.T) {
	lines := patternLines{{id: "ns1/sec/haproxy-*:fullpem:/certs/*.pem"}}
	_, ok := lines.line("ns1/sec/haproxy-*:fullpem:/certs/*.pem@ns1/sec/haproxy-a")
	assert.True(t, ok, "the item of a datastore of the line")
	_, ok = lines.line("ns1/sec/haproxy:fullpem:/certs/a.pem")
	assert.False(t, ok, "a line naming its datastore")
}

// The part of a datastore name the * matched names a file or a directory
// where the line says, never the one above.
func TestExpandDestStaysWhereTheLineSays(t *testing.T) {
	for _, c := range []struct{ dest, match, want string }{
		{"/certs/*.pem", "www.example.com", "/certs/www.example.com.pem"},
		{"/certs/*.pem", "..", "/certs/...pem"},
		{"/tenants/*/", "a", "/tenants/a/"},
		{"/tenants/*/", "..", ""},
		{"/tenants/*", ".", ""},
		{"/tenants/x*/", "..", "/tenants/x../"},
	} {
		got, ok := expandDest(c.dest, c.match)
		assert.Equal(t, c.want, got, c.dest+" "+c.match)
		assert.Equal(t, c.want != "", ok, c.dest+" "+c.match)
	}
}

func TestValidateInstall(t *testing.T) {
	for value, want := range map[string]string{
		"/certs/*.pem from ./sec/haproxy-* key fullpem signal HUP:container#1": "",
		"/tenants/*/ from ns1/cfg/tenant-* key conf/*":                         "",
		"/a from ./cfg/c key k /b/ /c/*.pem from cfg/x-*":                      "",
		"/certs/ from ./sec/haproxy-* key fullpem":                             "holds one *",
		"/certs/*/*.pem from ./sec/haproxy-* key fullpem":                      "holds one *",
		"/certs/*.pem from ./sec/*-*":                                          "holds one *",
		"/certs/*.pem from ./svc/haproxy-*":                                    "sec or cfg",
		"/certs/*.pem from ./sec/Haproxy-*":                                    "uppercase",
	} {
		err := ValidateInstall(value)
		if want == "" {
			assert.NoError(t, err, value)
		} else {
			assert.ErrorContains(t, err, want, value)
		}
	}
}
