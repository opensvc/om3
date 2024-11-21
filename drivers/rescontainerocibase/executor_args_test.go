package rescontainerocibase

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
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
			"newOpt2",
		},
	}

	ea := ExecutorArg{
		BT:                     bt,
		RunArgsDNSOptionOption: "--dns-option",
	}

	if err := bt.Configure(); err != nil {
		require.NoError(t, err)
	}

	base, err := ea.RunArgsBase()
	if err != nil {
		require.NoError(t, err)
	}

	expected := []string{
		"container", "run", "--name", "foo.id1",
		"--hostname", "node1",
		"--privileged",
		"--net", "host",
		"--newOpt1", "newOpt1Value",
		"newOpt2",
	}

	base.DropOptionAndAnyValue("--label")
	base.DropOptionAndAnyValue("-e")

	require.ElementsMatchf(t, expected, base.Get(), "want: %s\ngot:  %s", expected, base.Get())
}
