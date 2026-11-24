package object

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

func TestNodeWake(t *testing.T) {
	env := testhelper.Setup(t)
	t.Cleanup(hostname.Impersonate("n1"))
	require.NoError(t, os.WriteFile(filepath.Join(env.Root, "etc", "cluster.conf"), []byte(`
[cluster]
name = test
nodes = n1 n2 n3 n4 n5 n6

[wake#n2]
mac = 22:23:24:2d:01:12

[wake#n3]
mac = 22:23:24:2d:01:13 22:23:24:2d:02:13
broadcast = 10.45.2.255
port = 7 9

[wake#n6]
mac = 22:23:24:2d:01:16,22:23:24:2d:02:16,
broadcast = 10.45.1.255,
port = 7,9

[wake#n5]
mac = 22:23:24:2d:01:15
port = nine
`), 0o600))

	var sent []string
	savedSend, savedLookup, savedBroadcastFor := wolSend, wakeLookup, wakeBroadcastFor
	t.Cleanup(func() { wolSend, wakeLookup, wakeBroadcastFor = savedSend, savedLookup, savedBroadcastFor })
	wolSend = func(mac, broadcast string, port int) error {
		sent = append(sent, fmt.Sprintf("%s %s %d", mac, broadcast, port))
		return nil
	}
	wakeLookup = func(name string) (net.IP, time.Duration, error) {
		switch name {
		case "n2":
			return net.ParseIP("10.45.1.12"), 0, nil
		case "n4":
			return net.ParseIP("10.99.0.14"), time.Hour, nil
		}
		return nil, 0, errors.New("no such host")
	}
	wakeBroadcastFor = func(ip net.IP) (net.IP, error) {
		if ip.String() == "10.45.1.12" {
			return net.ParseIP("10.45.1.255"), nil
		}
		return nil, fmt.Errorf("no local interface on the subnet of %s", ip)
	}

	n, err := NewNode()
	require.NoError(t, err)

	t.Run("the broadcast of the local interface on the node subnet, port 9", func(t *testing.T) {
		sent = nil
		require.NoError(t, n.Wake("n2", nil))
		assert.Equal(t, []string{"22:23:24:2d:01:12 10.45.1.255 9"}, sent)
	})

	t.Run("the configured broadcasts and ports", func(t *testing.T) {
		sent = nil
		require.NoError(t, n.Wake("n3", nil))
		assert.Equal(t, []string{
			"22:23:24:2d:01:13 10.45.2.255 7",
			"22:23:24:2d:01:13 10.45.2.255 9",
			"22:23:24:2d:02:13 10.45.2.255 7",
			"22:23:24:2d:02:13 10.45.2.255 9",
		}, sent)
	})

	t.Run("commas separate the list words too", func(t *testing.T) {
		sent = nil
		require.NoError(t, n.Wake("n6", nil))
		assert.Equal(t, []string{
			"22:23:24:2d:01:16 10.45.1.255 7",
			"22:23:24:2d:01:16 10.45.1.255 9",
			"22:23:24:2d:02:16 10.45.1.255 7",
			"22:23:24:2d:02:16 10.45.1.255 9",
		}, sent)
	})

	t.Run("the macs argument replaces the configured ones", func(t *testing.T) {
		sent = nil
		require.NoError(t, n.Wake("n2", []string{"22:23:24:2d:00:12"}))
		assert.Equal(t, []string{"22:23:24:2d:00:12 10.45.1.255 9"}, sent)
	})

	t.Run("the local node is awake", func(t *testing.T) {
		sent = nil
		require.NoError(t, n.Wake("n1", nil))
		assert.Empty(t, sent)
	})

	t.Run("errors", func(t *testing.T) {
		cases := map[string]struct {
			nodename string
			macs     []string
			want     string
		}{
			"not a cluster node":        {"n9", nil, "not a cluster node"},
			"no mac":                    {"n4", nil, "set wake#n4.mac"},
			"no local interface on lan": {"n4", []string{"22:23:24:2d:01:14"}, "set wake#n4.broadcast"},
			"invalid port":              {"n5", nil, "invalid port"},
			"empty node name":           {"", nil, "not set"},
		}
		for name, c := range cases {
			sent = nil
			err := n.Wake(c.nodename, c.macs)
			assert.ErrorContains(t, err, c.want, name)
			assert.Empty(t, sent, name)
		}
	})
}

// The wake#<nodename> keywords are known to the cluster and the node
// configurations, so set, get and validate accept them.
func TestNodeWakeKeywords(t *testing.T) {
	for _, option := range []string{"mac", "broadcast", "port", "relays"} {
		k := key.New("wake#n2", option)
		assert.NotNil(t, ccfgKeywordStore.Lookup(k, naming.KindCcfg, ""), "cluster %s", k)
		assert.NotNil(t, NodeKeywordStore.Lookup(k, naming.KindInvalid, ""), "node %s", k)
	}
}
