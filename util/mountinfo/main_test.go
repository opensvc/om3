package mountinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	b := []byte(`22 1 252:0 / / rw,relatime shared:1 - ext4 /dev/mapper/root-root rw,errors=remount-ro
480 22 7:1 /data /srv/dev\040btrfs rw,relatime shared:250 - btrfs /dev/loop1 rw,space_cache=v2,subvolid=289,subvol=/data
500 22 0:52 / /tank/x rw shared:260 - zfs tank/x rw,xattr
garbage
`)
	assert.Equal(t, []Entry{
		{Root: "/", Target: "/", FSType: "ext4", Source: "/dev/mapper/root-root", Options: "rw,relatime", SuperOptions: "rw,errors=remount-ro"},
		{Root: "/data", Target: "/srv/dev btrfs", FSType: "btrfs", Source: "/dev/loop1", Options: "rw,relatime", SuperOptions: "rw,space_cache=v2,subvolid=289,subvol=/data"},
		{Root: "/", Target: "/tank/x", FSType: "zfs", Source: "tank/x", Options: "rw", SuperOptions: "rw,xattr"},
	}, Parse(b))
}
