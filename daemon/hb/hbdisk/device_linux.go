//go:build linux

package hbdisk

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/ncw/directio"
)

// openDevice opens the specified block device in Direct IO mode.
// It validates that the path follows a static naming convention for safety and
// returns an error on failure.
func (t *device) openDevice(newDev string) error {
	if strings.HasPrefix("/dev/dm-", t.path) {
		return fmt.Errorf("%s is not static enough a name to allow. please use a /dev/mapper/<name> or /dev/by-<attr>/<value> dev path", t.path)
	}
	if strings.HasPrefix("/dev/sd", t.path) {
		return fmt.Errorf("%s is not a static name. using a /dev/mapper/<name> or /dev/by-<attr>/<value> dev path is safer", t.path)
	}

	err := t.ensureBlockDevice(newDev)
	if err != nil {
		return err
	}
	t.mode = modeDirectIO
	if t.file, err = directio.OpenFile(newDev, os.O_RDWR|os.O_SYNC|syscall.O_DSYNC, openDevicePermission); err != nil {
		return fmt.Errorf("%s directio open block device: %w", newDev, err)
	}

	return nil
}
