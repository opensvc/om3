package rescontainerocibase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/drivers/rescontainer"
)

func TestExecutorArg_RunArgsBase(t *testing.T) {
	p := naming.Path{Name: "foo", Kind: naming.KindSvc}

	bt := &BT{
		T: resource.T{ResourceID: &resourceid.T{
			Name: "id1",
		}},
		Path:       p,
		Hostname:   "node1",
		Privileged: true,
		NetNS:      "host",
		RunArgs: []string{
			"-n", "fooX", "--detach", "--privileged",
			"--newOpt1", "newOpt1Value",
			"-h", "nodeX", "--hostname", "nodeY", "--net", "netValue1",
			"--network", "netValue2",
			"-v", "/etc/localtime:/etc/localtime:ro",
			"newOpt2",
		},
	}

	ea := ExecutorArg{
		BT: bt,
	}

	if err := bt.Configure(); err != nil {
		require.NoError(t, err)
	}

	base, err := ea.RunArgsBase(context.Background())
	if err != nil {
		require.NoError(t, err)
	}

	expected := []string{
		"container", "run", "--name", "foo.id1",
		"--hostname", "node1",
		"--privileged",
		"--net", "host",
		"--detach",
		"--newOpt1", "newOpt1Value",
		"-v", "/etc/localtime:/etc/localtime:ro",
		"newOpt2",
	}

	base.DropOptionAndAnyValue("--label")
	base.DropOptionAndAnyValue("-e")

	require.ElementsMatchf(t, expected, base.Get(), "want: %s\ngot:  %s", expected, base.Get())
}

// Podman stops reading options at the first argument that is not one, so the
// name of the container comes last or the options read as more names.
func TestLogsArgsPutTheContainerNameLast(t *testing.T) {
	ea := &ExecutorArg{BT: &BT{Name: "c1"}}
	require.Equal(t,
		[]string{"container", "logs", "--follow", "--tail", "3", "c1"},
		ea.LogsArgs(true, 3).Get())
	require.Equal(t,
		[]string{"container", "logs", "c1"},
		ea.LogsArgs(false, 0).Get())
}

// A container is given a resolv.conf when a nameserver is named, by the
// cluster or by its dns keyword, and none otherwise: the search list of the
// object alone made a file naming no nameserver, and the container resolved
// no name at all, where the engine gives it the resolver of the host.
func TestResolvConfMount(t *testing.T) {
	newArg := func(dns, dnsExtra []string) (*ExecutorArg, *[]string) {
		var written []string
		bt := &BT{
			T:            resource.T{ResourceID: &resourceid.T{Name: "container#1"}},
			Path:         naming.Path{Namespace: "bug", Name: "dns", Kind: naming.KindSvc},
			ObjectDomain: "bug.svc.demov3",
			DNS:          dns,
			DNSExtra:     dnsExtra,
		}
		ea := &ExecutorArg{
			BT: bt,
			WriteResolvConf: func(resolvConf rescontainer.ResolvConf) (string, error) {
				written = append(written, resolvConf.String())
				return "/var/lib/opensvc/resolv.conf", nil
			},
		}
		return ea, &written
	}

	t.Run("no nameserver", func(t *testing.T) {
		ea, written := newArg(nil, nil)
		mount, err := ea.resolvConfMount()
		require.NoError(t, err)
		require.Empty(t, mount, "the engine resolver is kept")
		require.Empty(t, *written, "nothing is written")
	})

	t.Run("the dns keyword", func(t *testing.T) {
		ea, written := newArg(nil, []string{"127.0.0.53"})
		mount, err := ea.resolvConfMount()
		require.NoError(t, err)
		require.Equal(t, "/var/lib/opensvc/resolv.conf:/etc/resolv.conf:ro", mount)
		require.Len(t, *written, 1)
		require.Contains(t, (*written)[0], "nameserver 127.0.0.53\n")
		require.Contains(t, (*written)[0], "search bug.svc.demov3 svc.demov3 demov3\n")
	})

	t.Run("the cluster dns", func(t *testing.T) {
		ea, written := newArg([]string{"10.29.0.11"}, nil)
		mount, err := ea.resolvConfMount()
		require.NoError(t, err)
		require.NotEmpty(t, mount)
		require.Contains(t, (*written)[0], "nameserver 10.29.0.11\n")
	})
}
