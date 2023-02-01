package osagentservice

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJoin(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	require.Nil(t, Join())
}
