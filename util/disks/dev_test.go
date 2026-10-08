package disks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDevDerivesThePathAsLsblkDoes(t *testing.T) {
	// Lines of "lsblk -o NAME,KNAME,WWN,SIZE,VENDOR,MODEL,TYPE,MAJ:MIN -b
	// -e7 --pairs" of the util-linux 2.23 of rhel 7, whose lsblk has no PATH
	// column, and the paths the PATH column of a later lsblk says.
	cases := map[string]struct {
		line string
		want Dev
	}{
		"disk": {
			line: `NAME="sda" KNAME="sda" WWN="0x600140550b8e30adba142d5b5fee019f" SIZE="1073741824" VENDOR="LIO-ORG " MODEL="c29_disk10      " TYPE="disk" MAJ:MIN="8:0"`,
			want: Dev{Name: "sda", WWN: "600140550b8e30adba142d5b5fee019f", Number: "8:0", Path: "/dev/sda", Size: 1073741824, Vendor: "LIO-ORG ", Model: "c29_disk10      ", Type: "disk"},
		},
		"partition": {
			line: `NAME="nvme0n1p1" KNAME="nvme0n1p1" WWN="" SIZE="536870912" VENDOR="" MODEL="" TYPE="part" MAJ:MIN="259:1"`,
			want: Dev{Name: "nvme0n1p1", Number: "259:1", Path: "/dev/nvme0n1p1", Size: 536870912, Type: "part"},
		},
		"multipath": {
			line: `NAME="360014052acd7e8d5a3f4cbca0915f28f" KNAME="dm-18" WWN="" SIZE="1073741824" VENDOR="" MODEL="" TYPE="mpath" MAJ:MIN="252:18"`,
			want: Dev{Name: "360014052acd7e8d5a3f4cbca0915f28f", Number: "252:18", Path: "/dev/mapper/360014052acd7e8d5a3f4cbca0915f28f", Size: 1073741824, Type: "mpath"},
		},
		"logical volume": {
			line: `NAME="root-root" KNAME="dm-0" WWN="" SIZE="50465865728" VENDOR="" MODEL="" TYPE="lvm" MAJ:MIN="252:0"`,
			want: Dev{Name: "root-root", Number: "252:0", Path: "/dev/mapper/root-root", Size: 50465865728, Type: "lvm"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, parseDev(c.line))
		})
	}
}
