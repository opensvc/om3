package lvm2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	tc = map[string]int64{
		"0 ":   0,
		"10k":  10 * 1024,
		"1m":   1 * 1024 * 1024,
		"1g":   1 * 1024 * 1024 * 1024,
		"4.4g": (4400 * 1024 * 1024 * 1024) / 1000,
	}
)

func TestVGInfo_Size(t *testing.T) {
	for vgFree, expected := range tc {
		vgInfo := VGInfo{VGFree: vgFree}
		size, err := vgInfo.Free()
		require.Nil(t, err)
		require.Equal(t, expected, size)
	}
}

func TestVGInfo_Free(t *testing.T) {
	for vgSize, expected := range tc {
		vgInfo := VGInfo{VGSize: vgSize}
		size, err := vgInfo.Free()
		require.Nil(t, err)
		require.Equal(t, expected, size)
	}
}
