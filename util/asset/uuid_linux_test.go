//go:build linux

package asset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachineUUID(t *testing.T) {
	saved := serialSources
	t.Cleanup(func() { serialSources = saved })
	for _, tc := range []struct {
		name    string
		content *string
		want    string
	}{
		{name: "kvm guest", content: ptr("c5f98577-2918-4da7-b7c9-0ae24687a572\n"), want: "c5f98577-2918-4da7-b7c9-0ae24687a572"},
		{name: "uppercase", content: ptr("C5F98577-2918-4DA7-B7C9-0AE24687A572\n"), want: "c5f98577-2918-4da7-b7c9-0ae24687a572"},
		{name: "unset by the firmware", content: ptr("00000000-0000-0000-0000-000000000000\n")},
		{name: "unsettable", content: ptr("ffffffff-ffff-ffff-ffff-ffffffffffff\n")},
		{name: "placeholder", content: ptr("Not Settable\n")},
		{name: "empty", content: ptr("\n")},
		{name: "no file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serialSources.dmiDir = t.TempDir()
			if tc.content != nil {
				require.NoError(t, os.WriteFile(filepath.Join(serialSources.dmiDir, "product_uuid"), []byte(*tc.content), 0400))
			}
			got, err := machineUUID()
			if tc.want == "" {
				assert.ErrorIs(t, err, ErrIgnore)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func ptr(s string) *string { return &s }
