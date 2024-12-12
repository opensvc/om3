package ressyncsymsrdfs

import (
	"encoding/xml"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	/*
		drv := T{
			SymID: "000000000193",
			SymDG: "DG1",
			RDFG:  5,
		}
	*/
	t.Run("symdg_x_list_ld", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/symdg_x_list_ld")
		require.Nil(t, err)
		var data XDGListLD
		err = xml.Unmarshal(b, &data)
		require.Nil(t, err)
		require.Equal(t, "003AD", data.DG.Devices[0].DevInfo.DevName)
	})
	t.Run("listPD", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/syminq_identifier_device_name")
		require.Nil(t, err)
		var data XInqIdentifierDeviceName
		err = xml.Unmarshal(b, &data)
		require.Nil(t, err)
		require.Equal(t, "003AD", data.Inquiries[0].DevInfo.DevName)
	})
}
