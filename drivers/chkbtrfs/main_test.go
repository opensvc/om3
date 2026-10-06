package chkbtrfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	out := "[/dev/loop0].write_io_errs    0\n[/dev/loop0].read_io_errs     3\n[/dev/loop0].corruption_errs  0\n"
	assert.Equal(t, []stat{
		{dev: "/dev/loop0", name: "write_io_errs", value: 0},
		{dev: "/dev/loop0", name: "read_io_errs", value: 3},
		{dev: "/dev/loop0", name: "corruption_errs", value: 0},
	}, parse([]byte(out)))
}
