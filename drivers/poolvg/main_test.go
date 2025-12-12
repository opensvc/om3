package poolvg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/lvm2"
)

func TestVGInfo_Free(t *testing.T) {
	vgInfo := lvm2.VGInfo{VGFree: "0 "}
	size, err := vgInfo.Free()
	require.Nil(t, err)
	require.Equal(t, int64(0), size)
}
