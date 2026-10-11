package datarecv

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := recv.render("k", []byte(tc.tmpl))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(b))
		})
	}
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
