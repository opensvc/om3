package listener

import (
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/testhelper"
)

func Test_getClusterName(t *testing.T) {
	cases := map[string]string{
		"cluster-with-name":    "test-cluster",
		"cluster-without-name": "default",
	}
	for f, expected := range cases {
		t.Run(f, func(t *testing.T) {
			env := testhelper.Setup(t)
			env.InstallFile("./testdata/"+f+".conf", "etc/cluster.conf")
			rawconfig.LoadSections()

			name, err := getClusterName()
			require.NoError(t, err, "getClusterName returns error")
			require.Equal(t, expected, name, "getClusterName returns unexpected cluster name")
		})
	}
}
