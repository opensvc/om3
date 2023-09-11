package osagentservice

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/rawconfig"
)

func TestJoin(t *testing.T) {
	rawconfig.Load(map[string]string{}) // for capabilities cache file
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	err := Join()
	if err != nil {
		require.ErrorContains(t, err, "cgroup deleted")
	}
}
