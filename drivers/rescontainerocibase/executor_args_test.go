package rescontainerocibase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
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
