package daemonapi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
)

func TestAnyToStrings(t *testing.T) {
	t.Run("converts the shapes an evaluated list keyword reaches us as", func(t *testing.T) {
		for name, tc := range map[string]struct {
			value any
			want  []string
		}{
			"json decoded list": {value: []any{"node1", "node2"}, want: []string{"node1", "node2"}},
			"empty json list":   {value: []any{}, want: []string{}},
			"go list":           {value: []string{"node1"}, want: []string{"node1"}},
			"space separated":   {value: "node1 node2", want: []string{"node1", "node2"}},
			"empty string":      {value: "", want: []string{}},
		} {
			t.Run(name, func(t *testing.T) {
				l, ok := anyToStrings(tc.value)
				assert.True(t, ok)
				assert.Equal(t, tc.want, l)
			})
		}
	})

	t.Run("refuses a value it can not read as a node list", func(t *testing.T) {
		for name, value := range map[string]any{
			"number":       float64(1),
			"mixed list":   []any{"node1", float64(2)},
			"object":       map[string]any{"node": "node1"},
			"nil":          nil,
			"bool":         true,
			"nested lists": []any{[]any{"node1"}},
		} {
			t.Run(name, func(t *testing.T) {
				_, ok := anyToStrings(value)
				assert.False(t, ok)
			})
		}
	})
}

func TestJoinAddrHost(t *testing.T) {
	t.Run("extracts the host of the location formats the client accepts", func(t *testing.T) {
		for addr, want := range map[string]string{
			"node1":                      "node1",
			"node1:1215":                 "node1",
			"node1.example.com:1215":     "node1.example.com",
			"https://node1:1215":         "node1",
			"https://node1.example.com":  "node1.example.com",
			"tls://node1:1215":           "node1",
			"1.2.3.4:1215":               "1.2.3.4",
			"https://1.2.3.4":            "1.2.3.4",
			"https://[2001:db8::1]:1215": "2001:db8::1",
		} {
			t.Run(addr, func(t *testing.T) {
				got, err := joinAddrHost(addr)
				require.NoError(t, err)
				assert.Equal(t, want, got)
			})
		}
	})

	t.Run("refuses a location with no host", func(t *testing.T) {
		for _, addr := range []string{"", "https://", "://node1"} {
			t.Run(addr, func(t *testing.T) {
				_, err := joinAddrHost(addr)
				assert.Error(t, err)
			})
		}
	})
}

func TestIsLoopback(t *testing.T) {
	t.Run("detects the names the enrolled node can not reach us at", func(t *testing.T) {
		// The certificate always carries 127.0.0.1, so a join addr check that
		// misses it would bless the one address guaranteed not to work.
		for _, host := range []string{"127.0.0.1", "127.0.1.1", "::1", "localhost"} {
			assert.Truef(t, isLoopback(host), "isLoopback(%q)", host)
		}
	})

	t.Run("accepts a reachable name", func(t *testing.T) {
		for _, host := range []string{"node1", "node1.example.com", "1.2.3.4", "2001:db8::1"} {
			assert.Falsef(t, isLoopback(host), "isLoopback(%q)", host)
		}
	})
}

func TestOurCertName(t *testing.T) {
	t.Run("expands a wildcard with our own label", func(t *testing.T) {
		for name, tc := range map[string]struct {
			localhost string
			certName  string
			want      string
		}{
			"short nodename":      {localhost: "node2", certName: "*.example.com", want: "node2.example.com"},
			"qualified nodename":  {localhost: "node2.example.com", certName: "*.example.com", want: "node2.example.com"},
			"another domain":      {localhost: "node2", certName: "*.other.com", want: "node2.other.com"},
			"single label domain": {localhost: "node2", certName: "*.lan", want: "node2.lan"},
		} {
			t.Run(name, func(t *testing.T) {
				a := &DaemonAPI{localhost: tc.localhost}
				assert.Equal(t, tc.want, a.ourCertName(tc.certName))
			})
		}
	})

	t.Run("keeps a plain name that designates us", func(t *testing.T) {
		for name, tc := range map[string]struct {
			localhost string
			certName  string
			want      string
		}{
			"same spelling":    {localhost: "node2", certName: "node2", want: "node2"},
			"both qualified":   {localhost: "node2.example.com", certName: "node2.example.com", want: "node2.example.com"},
			"we are qualified": {localhost: "node2.example.com", certName: "node2", want: "node2"},
		} {
			t.Run(name, func(t *testing.T) {
				a := &DaemonAPI{localhost: tc.localhost}
				assert.Equal(t, tc.want, a.ourCertName(tc.certName))
			})
		}
	})

	t.Run("refuses a name designating another node", func(t *testing.T) {
		// The certificate is cluster wide, so its plain names are usually
		// those of the node that bootstrapped the cluster.
		for name, tc := range map[string]struct {
			localhost string
			certName  string
		}{
			"peer short name":              {localhost: "node2", certName: "node1"},
			"peer qualified name":          {localhost: "node2", certName: "node1.example.com"},
			"our name qualified elsewhere": {localhost: "node2", certName: "node2.example.com"},
			"an ip":                        {localhost: "node2", certName: "1.2.3.4"},
			"empty localhost":              {localhost: "", certName: "*.example.com"},
			"bare wildcard":                {localhost: "node2", certName: "*."},
		} {
			t.Run(name, func(t *testing.T) {
				a := &DaemonAPI{localhost: tc.localhost}
				assert.Equal(t, "", a.ourCertName(tc.certName))
			})
		}
	})
}

func TestPortToJoin(t *testing.T) {
	const localhost = "node2"

	setup := func(t *testing.T) *DaemonAPI {
		t.Helper()
		daemonsubsystem.InitData()
		cluster.InitData()
		t.Cleanup(func() {
			daemonsubsystem.InitData()
			cluster.InitData()
		})
		return &DaemonAPI{localhost: localhost}
	}

	t.Run("localPort prefers the port our listener bound", func(t *testing.T) {
		a := setup(t)
		daemonsubsystem.DataListener.Set(localhost, &daemonsubsystem.Listener{Port: "1216"})
		cluster.ConfigData.Set(&cluster.Config{Listener: cluster.ConfigListener{Port: 1217}})
		assert.Equal(t, "1216", a.localPort())
	})

	t.Run("localPort falls back to the configured port", func(t *testing.T) {
		for name, lsnr := range map[string]*daemonsubsystem.Listener{
			"no listener data": nil,
			"empty port":       {Port: ""},
		} {
			t.Run(name, func(t *testing.T) {
				a := setup(t)
				if lsnr != nil {
					daemonsubsystem.DataListener.Set(localhost, lsnr)
				}
				cluster.ConfigData.Set(&cluster.Config{Listener: cluster.ConfigListener{Port: 1217}})
				assert.Equal(t, "1217", a.localPort())
			})
		}
	})

	t.Run("localPort ignores the listener of a peer", func(t *testing.T) {
		// That port is the peer's own, and says nothing about ours.
		a := setup(t)
		daemonsubsystem.DataListener.Set("node1", &daemonsubsystem.Listener{Port: "1216"})
		cluster.ConfigData.Set(&cluster.Config{Listener: cluster.ConfigListener{Port: 1217}})
		assert.Equal(t, "1217", a.localPort())
	})

	t.Run("clusterPort never reads a bound port", func(t *testing.T) {
		// A name designating a peer must not be handed the port we bound.
		setup(t)
		daemonsubsystem.DataListener.Set(localhost, &daemonsubsystem.Listener{Port: "1216"})
		cluster.ConfigData.Set(&cluster.Config{Listener: cluster.ConfigListener{Port: 1217}})
		assert.Equal(t, "1217", clusterPort())
	})

	t.Run("falls back to the default port", func(t *testing.T) {
		a := setup(t)
		want := fmt.Sprint(daemonenv.HTTPPort)
		assert.Equal(t, want, clusterPort())
		assert.Equal(t, want, a.localPort())
	})
}

func TestJoinURL(t *testing.T) {
	for name, tc := range map[string]struct {
		host string
		port string
		want string
	}{
		"name":          {host: "node2.example.com", port: "1215", want: "https://node2.example.com:1215"},
		"ipv4":          {host: "1.2.3.4", port: "1216", want: "https://1.2.3.4:1216"},
		"ipv6bracketed": {host: "2001:db8::1", port: "1215", want: "https://[2001:db8::1]:1215"},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, joinURL(tc.host, tc.port))
		})
	}
}
