//go:build !linux

package scsi

import "github.com/opensvc/om3/v3/util/device"

// currentNexus tells no nexus on the operating systems not linux.
func currentNexus(device.T) ([]nexus, int, error) {
	return nil, 0, ErrNoNexus
}
