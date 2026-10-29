//go:build linux

package resdiskvg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/device"
)

type fakePVRemover struct {
	VGDriver
	removed []string
}

func (t *fakePVRemover) RemovePVs(_ context.Context, pvs device.L) error {
	for _, pv := range pvs {
		t.removed = append(t.removed, pv.Path())
	}
	return nil
}

// TestRemovePVs checks the physical volumes of a group removed are unlabeled,
// but the ones lvm misses, which have no device.
func TestRemovePVs(t *testing.T) {
	vg := &fakePVRemover{}
	pvs := device.L{
		device.New("/dev/mapper/36001405812e318152c24813a8ee571bd"),
		device.New("[unknown]"),
		device.New("/dev/vdc"),
	}
	r := &T{}
	require.NoError(t, r.removePVs(context.Background(), vg, pvs))
	require.Equal(t, []string{"/dev/mapper/36001405812e318152c24813a8ee571bd", "/dev/vdc"}, vg.removed)
}
