package zfs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/funcopt"
)

func TestFsDestroyArgs(t *testing.T) {
	args := func(fopts ...funcopt.O) []string {
		opts := &fsDestroyOpts{Name: "pool/fs@snap"}
		funcopt.Apply(opts, fopts...)
		return fsDestroyOptsToArgs(*opts)
	}
	require.Equal(t, []string{"destroy", "pool/fs@snap"}, args(FilesystemDestroyWithRemoveSnapshots(false)))
	require.Equal(t, []string{"destroy", "-r", "pool/fs@snap"}, args(FilesystemDestroyWithRemoveSnapshots(true)))
}
