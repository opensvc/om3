package datarecv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	_ "github.com/opensvc/om3/v3/drivers/networkbridge"
	_ "github.com/opensvc/om3/v3/drivers/networklo"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

type templateReceiver struct {
	o any
}

func (r templateReceiver) Head() string                     { return "" }
func (r templateReceiver) Log() *plog.Logger                { return plog.NewLogger(zerolog.Nop()) }
func (r templateReceiver) StatusLog() resource.StatusLogger { return resource.NewStatusLog() }
func (r templateReceiver) GetObject() any                   { return r.o }
func (r templateReceiver) RID() string                      { return "volume#1" }

// A template reads the env section of the object, the references evaluated
// for this node, and the keys of a datastore the object may read.
func TestRender(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "web", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n[env]\nport = 80\nnames = a b\n[volume#1]\nname = {name}\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)

	cfg := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	ds, err := object.NewDataStore(cfg)
	require.NoError(t, err)
	require.NoError(t, ds.AddKey("psk", []byte("s3cr3t")))

	recv := &DataRecv{to: templateReceiver{o: o}}
	for _, tc := range []struct {
		name    string
		tmpl    string
		want    string
		wantErr bool
	}{
		{name: "env", tmpl: "port={{.port}}", want: "port=80"},
		{name: "ref", tmpl: `{{ref "nodename"}} {{ref "name"}} {{ref "env.port"}}`, want: hostname.Hostname() + " web 80"},
		{name: "split", tmpl: `{{range split .names}}[{{.}}]{{end}}`, want: "[a][b]"},
		{name: "key", tmpl: `secret = "{{key "./cfg/web" "psk"}}"`, want: `secret = "s3cr3t"`},
		{name: "missing env", tmpl: "{{.nope}}", wantErr: true},
		{name: "unknown ref", tmpl: `{{ref "nope"}}`, wantErr: true},
		{name: "missing key", tmpl: `{{key "./cfg/web" "nope"}}`, wantErr: true},
		{name: "not a datastore", tmpl: `{{key "./svc/web" "psk"}}`, wantErr: true},
		{name: "reverse zones", tmpl: `{{with network "default"}}{{range .ReverseZones}}{{.}} {{end}}{{end}}`, want: "22.10.in-addr.arpa "},
		{name: "no reverse zone of lo", tmpl: `{{with network "lo"}}[{{range .ReverseZones}}{{.}}{{end}}]{{end}}`, want: "[]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := recv.render("k", []byte(tc.tmpl), &[]templateKey{})
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(b))
		})
	}
}

// A cluster whose nameservers do not answer the reverse lookups of its
// addresses has networks with no reverse zone: a resolver forwarding them to
// the cluster nameservers would not be answered.
func TestRenderNoReverseZones(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	c, err := object.NewCluster()
	require.NoError(t, err)
	require.NoError(t, c.Config().Set(keyop.ParseOps([]string{"cluster.dns_reverse=false"})...))
	require.NoError(t, c.Config().Commit())

	p := naming.Path{Name: "web", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)
	recv := &DataRecv{to: templateReceiver{o: o}}
	b, err := recv.render("k", []byte(`{{with network "default"}}[{{range .ReverseZones}}{{.}}{{end}}]{{end}}`), &[]templateKey{})
	require.NoError(t, err)
	assert.Equal(t, "[]", string(b))
}

// A template line renders, the others install the key as it is.
func TestParseInstallTemplate(t *testing.T) {
	p := naming.Path{Name: "web", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	recv := &DataRecv{Install: []string{
		"/a.conf", "from", "./cfg/web", "key", "a.conf", "template",
		"/b.conf", "from", "./cfg/web", "key", "b.conf",
	}}
	_, files := recv.parseInstall("/head", p)
	require.Len(t, files, 2)
	assert.NotNil(t, files[0].Render)
	assert.Nil(t, files[1].Render)
}

// The record of an install replaces the previous one whole, and leaves no
// temporary file in the directory a watcher reads.
func TestWriteInstalled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "installed")
	require.NoError(t, writeInstalled(dir, "fs#1", []string{"/a.conf"}))
	require.NoError(t, writeInstalled(dir, "fs#1", []string{"/b.conf", "/c.conf"}))
	v, err := ReadInstalled(filepath.Join(dir, "fs#1"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/b.conf", "/c.conf"}, v.Files)
	assert.False(t, v.At.IsZero())
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

// Networks joining the same two addresses share a tunnel, listed once, and a
// peer the network is routed to is no tunnel.
func TestUniqueTunnels(t *testing.T) {
	nws := []templateNetwork{
		{Name: "a", Peers: []network.Peer{{Nodename: "n2", Tunnel: "otun1"}, {Nodename: "n3"}}},
		{Name: "b", Peers: []network.Peer{{Nodename: "n2", Tunnel: "otun1"}, {Nodename: "n3", Tunnel: "otun2"}}},
	}
	got := uniqueTunnels(nws)
	require.Len(t, got, 2)
	assert.Equal(t, "otun1", got[0].Tunnel)
	assert.Equal(t, "otun2", got[1].Tunnel)
}

// The keys a template read are matched by the datastore and the key changed,
// and by the datastore of the template reading them.
func TestTemplateKeys(t *testing.T) {
	cfg := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: "ns1"}
	sec := naming.Path{Name: "web", Kind: naming.KindSec, Namespace: "ns1"}
	other := naming.Path{Name: "other", Kind: naming.KindSec, Namespace: "ns1"}
	filename := filepath.Join(t.TempDir(), "fs#1", "template_keys")
	require.NoError(t, writeTemplateKeys(filename, templateKeys{
		templateID(cfg, "a.conf"): {{Path: sec.String(), Key: "psk"}},
	}))
	m := readTemplateKeys(filename)
	assert.True(t, m.readsFrom(sec, "psk"))
	assert.True(t, m.readsFrom(sec, ""))
	assert.False(t, m.readsFrom(sec, "other"))
	assert.False(t, m.readsFrom(other, ""))
	assert.True(t, m.reads(cfg, sec, ""))
	assert.False(t, m.reads(other, sec, ""))

	require.NoError(t, writeTemplateKeys(filename, templateKeys{}))
	assert.NoFileExists(t, filename)
}
