package osagentservice

import (
	"os"
	"testing"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/stretchr/testify/require"
)

func TestJoin(t *testing.T) {
	rawconfig.Load(map[string]string{}) // for capabilities cache file
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	require.Nil(t, Join())
}
