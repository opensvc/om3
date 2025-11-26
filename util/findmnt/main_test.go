package findmnt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMountInfo(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		expect []MountInfo
	}{
		{"simple mount", `TARGET="/" SOURCE="/dev/sda1" FSTYPE="ext4" OPTIONS="rw,relatime,errors=remount-ro"`,
			[]MountInfo{{Target: "/", Source: "/dev/sda1", FsType: "ext4", Options: "rw,relatime,errors=remount-ro"}}},

		{"multiple mounts", `TARGET="/sys" SOURCE="sysfs" FSTYPE="sysfs" OPTIONS="rw,nosuid,nodev,noexec,relatime"
TARGET="/proc" SOURCE="proc" FSTYPE="proc" OPTIONS="rw,nosuid,nodev,noexec,relatime"
TARGET="/dev" SOURCE="udev" FSTYPE="devtmpfs" OPTIONS="rw,nosuid,relatime,size=4058636k,nr_inodes=1014659,mode=755"`,
			[]MountInfo{
				{Target: "/sys", Source: "sysfs", FsType: "sysfs", Options: "rw,nosuid,nodev,noexec,relatime"},
				{Target: "/proc", Source: "proc", FsType: "proc", Options: "rw,nosuid,nodev,noexec,relatime"},
				{Target: "/dev", Source: "udev", FsType: "devtmpfs", Options: "rw,nosuid,relatime,size=4058636k,nr_inodes=1014659,mode=755"}}},

		{"simple mount with spaces", `TARGET="/media/Mon Disque" SOURCE="/dev/sdb1" FSTYPE="ext4" OPTIONS="rw,relatime"`,
			[]MountInfo{
				{Target: "/media/Mon Disque", Source: "/dev/sdb1", FsType: "ext4", Options: "rw,relatime"},
			}},

		{"multiple mounts with spaces", `TARGET="/media/user/USB Drive" SOURCE="/dev/sdc1" FSTYPE="vfat" OPTIONS="rw,nosuid,nodev,relatime,uid=1000,gid=1000"
TARGET="/mnt/nas/Documents partagés" SOURCE="nas.local:/volume1/documents" FSTYPE="nfs" OPTIONS="rw,relatime,vers=4.1"`,
			[]MountInfo{
				{Target: "/media/user/USB Drive", Source: "/dev/sdc1", FsType: "vfat", Options: "rw,nosuid,nodev,relatime,uid=1000,gid=1000"},
				{Target: "/mnt/nas/Documents partagés", Source: "nas.local:/volume1/documents", FsType: "nfs", Options: "rw,relatime,vers=4.1"}}},

		{"Invalid format", `TARGT="/mnt/data" SOUCE="/dev/sdb1" FSTYPE="ext4" OPTIONS="rw,relatime"`, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMountInfo([]byte(tc.input))
			require.Equal(t, tc.expect, got)
			if tc.expect == nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
