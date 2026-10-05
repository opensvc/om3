package chkraid

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/check"
)

func TestParseLdPdInfo(t *testing.T) {
	out := "Adapter #0\n\nNumber of Virtual Disks: 1\nState: Optimal\nFirmware state: Online, Spun Up\nFirmware state: Rebuild\n" +
		"Adapter #1\nState: Degraded\n"
	assert.Equal(t, []check.Result{{Instance: "slot0", Value: 1}, {Instance: "slot1", Value: 1}}, parseLdPdInfo([]byte(out)))
}

func TestParseBbuStatus(t *testing.T) {
	out := "BBU status for Adapter: 0\n\nBatteryType: iBBU\nTemperature: 31 C\n  Relative State of Charge: 98 %\n  isSOHGood: Yes\n"
	assert.Equal(t, []check.Result{
		{Instance: "slot0 battery temp", Value: 31, Unit: "C"},
		{Instance: "slot0 battery charge", Value: 98, Unit: "%"},
		{Instance: "slot0 battery isSOHGood", Value: 0},
	}, parseBbuStatus([]byte(out)))
}

func TestParseSas2Display(t *testing.T) {
	out := "IR volume 1\n  Volume Name                             : Virtual Disk 1\n  Status of volume                        : Okay (OKY)\n" +
		"Device is a Hard disk\n  Enclosure #                             : 1\n  Slot #                                  : 0\n  State                                   : Optimal (OPT)\n" +
		"Device is a Hard disk\n  Enclosure #                             : 1\n  Slot #                                  : 1\n  State                                   : Failed (FLD)\n" +
		"Device is a Enclosure services device\n  Enclosure #                             : 1\n  State                                   : Standby (SBY)\n"
	assert.Equal(t, []check.Result{
		{Instance: "ctrl:0,LD1", Value: 0},
		{Instance: "ctrl:0,PD1:0", Value: 0},
		{Instance: "ctrl:0,PD1:1", Value: 1},
		{Instance: "ctrl:0,Enc1", Value: 0},
	}, parseDisplay("0", []byte(out)))
	assert.Equal(t, []string{"0", "1"}, controllers([]byte("Index    Type\n-----\n  0     SAS2008\n  1     SAS2308_2\n")))
}

func TestParseSmartArrayStatus(t *testing.T) {
	assert.Equal(t, []string{"0", "3"}, smartArraySlots([]byte("Smart Array P410i in Slot 0 (Embedded)\nSmart Array P812 in Slot 3\n")))
	out := "\n   logicaldrive 1 (136.7 GB, RAID 1): OK\n   physicaldrive 1I:1:2 (port 1I:box 1:bay 2, 146 GB): Failed\n"
	assert.Equal(t, []check.Result{
		{Instance: "logicaldrive 1 (136.7 gb, raid 1)", Value: 0},
		{Instance: "physicaldrive 1i:1:2 (port 1i:box 1:bay 2, 146 gb): failed", Value: 1},
	}, parseSmartArrayStatus([]byte(out)))
}
