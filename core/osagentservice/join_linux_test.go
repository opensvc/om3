package osagentservice

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestJoin(t *testing.T) {
	if os.Getpid() != 0 {
		t.Skip("skipped for non root user")
	}
	require.Nil(t, Join())
}
