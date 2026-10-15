//go:build linux

package asset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerial(t *testing.T) {
	saved := serialSources
	t.Cleanup(func() { serialSources = saved })
	for _, tc := range []struct {
		name    string
		dmi     map[string]string
		cpuinfo string
		want    string
	}{
		{name: "system serial", dmi: map[string]string{"product_serial": "DN9R4TT\n", "chassis_serial": "CH1\n"}, want: "DN9R4TT"},
		{name: "chassis serial when the system has none", dmi: map[string]string{"product_serial": "Not Specified\n", "chassis_serial": "CH1\n"}, want: "CH1"},
		{name: "chassis serial when the system one is empty", dmi: map[string]string{"product_serial": "\n", "chassis_serial": "CH1\n"}, want: "CH1"},
		{name: "chassis serial when the system one is zeroes", dmi: map[string]string{"product_serial": "0000000\n", "chassis_serial": "CH1\n"}, want: "CH1"},
		{name: "virtual machine without serial", dmi: map[string]string{"product_serial": "\n", "chassis_serial": "\n"}, want: "Not Specified"},
		{name: "no dmi, serial in cpuinfo", cpuinfo: "processor\t: 0\nSerial\t\t: 10000000abcdef01\n", want: "10000000abcdef01"},
		{name: "no dmi, no serial", cpuinfo: "processor\t: 0\n", want: "Unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			serialSources.dmiDir = filepath.Join(dir, "dmi")
			serialSources.cpuinfo = filepath.Join(dir, "cpuinfo")
			if tc.dmi != nil {
				require.NoError(t, os.MkdirAll(serialSources.dmiDir, 0755))
				for name, s := range tc.dmi {
					require.NoError(t, os.WriteFile(filepath.Join(serialSources.dmiDir, name), []byte(s), 0644))
				}
			}
			require.NoError(t, os.WriteFile(serialSources.cpuinfo, []byte(tc.cpuinfo), 0644))
			assert.Equal(t, tc.want, serial())
		})
	}
}
