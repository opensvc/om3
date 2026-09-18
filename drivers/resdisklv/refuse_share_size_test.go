package resdisklv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lvm2 computes a share of the volume group once, when the volume is created,
// and om never learns what it came out as. A resize writes the size it reached
// back into the keyword it grew, and a count of bytes written over "100%FREE"
// would answer a question nobody asked.
func TestRefuseShareSize(t *testing.T) {
	for _, size := range []string{"100%FREE", "50%VG", "20%PVS"} {
		t.Run(size, func(t *testing.T) {
			err := (&T{Size: size}).refuseShareSize()
			require.Error(t, err)
			assert.Contains(t, err.Error(), size, "the refusal names what is written")
			assert.Contains(t, err.Error(), "capacity", "and what to write instead")
		})
	}

	// A size, and a size om computed, are both sizes.
	for _, size := range []string{"10m", "1g", "1073741824", ""} {
		t.Run("size "+size, func(t *testing.T) {
			assert.NoError(t, (&T{Size: size}).refuseShareSize())
		})
	}
}
