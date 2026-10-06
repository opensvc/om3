package chkfszfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	out := "tank\t1048576\t3145728\t/tank\n" +
		"tank/a@snap\t0\t0\t-\n" +
		"tank/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\t1\t1\tlegacy\n" +
		"tank/osvc_sync_x\t1\t1\tnone\n" +
		"tank/b\t2048\t2048\tnone\n"
	assert.Equal(t, []dataset{
		{name: "tank", used: 1048576, avail: 3145728, mountPoint: "/tank"},
		{name: "tank/b", used: 2048, avail: 2048, mountPoint: "none"},
	}, parse([]byte(out)))
}
