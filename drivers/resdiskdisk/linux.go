//go:build linux

package resdiskdisk

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/yookoala/realpath"

	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/scsi"
	"opensvc.com/opensvc/util/udevadm"
)

func (t T) expectedDevPath() string {
	s := strings.ToLower(t.DiskID)
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	return s
}

func (t T) devPath() string {
	s := t.expectedDevPath()
	matches, err := filepath.Glob("/dev/disk/by-id/dm-uuid-mpath-[36]" + s)
	if err != nil || len(matches) != 1 {
		return ""
	}
	return matches[0]
}

func (t T) ExposedDevices() device.L {
	l := make(device.L, 0)
	p, err := realpath.Realpath(t.devPath())
	if err != nil {
		return l
	}
	l = append(l, device.New(p))
	return l
}

func (t *T) Status(ctx context.Context) status.T {
	if t.DiskID == "" {
		return status.NotApplicable
	}
	if t.devPath() == "" {
		t.StatusLog().Warn("%s does not exist", t.expectedDevPath())
		return status.Down
	}
	return status.NotApplicable
}

func (t T) unconfigure() error {
	for _, dev := range t.ExposedDevices() {
		slaves, err := dev.Slaves()
		if err != nil {
			return errors.Wrapf(err, "%s get slaves", dev)
		}
		for _, slave := range slaves {
			if err := slave.Delete(); err != nil {
				return errors.Wrapf(err, "%s slave %s delete", dev, slave)
			} else {
				t.Log().Info().Msgf("%s slave %s deleted", dev, slave)
			}
		}
		if err := dev.RemoveMultipath(); err != nil {
			return errors.Wrapf(err, "%s multipath remove", dev)
		} else {
			t.Log().Info().Msgf("%s multipath removed", dev)
		}
	}
	return nil
}

func (t T) configure(force forceMode) error {
	exposedDevices := t.ExposedDevices()
	if force == preserve && len(exposedDevices) > 0 {
		t.Log().Info().Msgf("system configuration: skip: device already exposed: %s", exposedDevices)
		return nil
	}
	if t.DiskID == "" {
		return errors.Errorf("system configuration: disk_id is not set")
	}
	t.Log().Info().Msg("system configuration: scsi scan")
	if err := scsi.LockedScanAll(10 * time.Second); err != nil {
		return errors.Wrap(err, "system configuration")
	}
	time.Sleep(2 * time.Second)
	udevadm.Settle()
	exposedDevices = t.ExposedDevices()
	if len(exposedDevices) == 0 {
		return errors.Errorf("system configuration: %s is not exposed device after scan", t.DiskID)
	}
	exposedDevice := exposedDevices[0]
	slaves, err := exposedDevice.Slaves()
	if err != nil {
		return errors.Wrap(err, "system configuration")
	}
	if len(slaves) < 1 {
		return errors.Errorf("system configuration: no slaves appeared for disk %s", exposedDevice.Path())
	}
	return nil
}
