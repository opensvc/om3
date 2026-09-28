package scheduler

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

func TestCheckRequire(t *testing.T) {
	st := instance.Status{Resources: instance.ResourceStatuses{
		"fs#1": resource.Status{Status: status.Down},
		"fs#2": resource.Status{Status: status.Up},
	}}
	require.NoError(t, checkRequire("fs#2(up)", st))
	require.ErrorContains(t, checkRequire("fs#1(up)", st), "fs#1 status is down")
	for i := 0; i < 20; i++ {
		// map order varies: the unmet condition must be reported
		// whichever is checked last
		require.ErrorContains(t, checkRequire("fs#1(up) fs#2(up)", st), "fs#1")
	}
	require.ErrorContains(t, checkRequire("fs#9(up)", st), "not found")
}
