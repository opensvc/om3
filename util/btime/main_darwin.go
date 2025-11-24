//go:build darwin

package btime

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

func bootTime() (uint64, error) {
	b, err := unix.SysctlRaw("kern.boottime")
	if err != nil {
		return 0, err
	}
	if len(b) < 8 {
		return 0, fmt.Errorf("kern.boottime returned %d bytes, expected at least 8", len(b))
	}
	sec := int64(binary.LittleEndian.Uint64(b[:8]))
	return uint64(sec), nil
}
