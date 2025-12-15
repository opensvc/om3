//go:build linux

package resdiskdisk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yookoala/realpath"

	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/scsi"
	"github.com/opensvc/om3/v3/util/udevadm"
)

func (t *T) expectedDevPath() string {
	s := strings.ToLower(t.DiskID)
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	return s
}

func (t *T) devPath() string {
	s := t.expectedDevPath()
	matches, err := filepath.Glob("/dev/disk/by-id/dm-uuid-mpath-[36]" + s)
	if err != nil || len(matches) != 1 {
		return ""
	}
	return matches[0]
}

func (t *T) ExposedDevices(ctx context.Context) device.L {
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

func (t *T) unconfigure(ctx context.Context) error {
	for _, dev := range t.ExposedDevices(ctx) {
		slaves, err := dev.Slaves()
		if err != nil {
			return fmt.Errorf("%s get slaves: %w", dev, err)
		}
		if err := dev.RemoveMultipath(ctx); err != nil {
			return fmt.Errorf("%s multipath remove: %w", dev, err)
		} else {
			t.Log().Infof("%s multipath removed", dev)
		}
		for _, slave := range slaves {
			if err := slave.Delete(); err != nil {
				return fmt.Errorf("%s slave %s delete: %w", dev, slave, err)
			} else {
				t.Log().Infof("%s slave %s deleted", dev, slave)
			}
		}
	}
	return nil
}

// waitAnyPath waits for the mapth device pointing to the disk id to appear.
func (t *T) waitDevPath(interval time.Duration, timeout time.Duration) error {
	limit := time.Now().Add(timeout)
	devPath := fmt.Sprintf("/dev/disk/by-id/dm-uuid-mpath-3%s", t.DiskID)
	for {
		if time.Now().After(limit) {
			break
		}
		dest, err := os.Readlink(devPath)
		if errors.Is(err, os.ErrNotExist) {
			time.Sleep(interval)
			continue
		}
		if err != nil {
			return err
		}
		if strings.Contains(dest, "/dm-") {
			t.Log().Infof("%s now exists", devPath)
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("timeout waiting for %s to appear", devPath)
}

// waitAnyPath waits for any sd or dm device pointing to the disk id to appear.
func (t *T) waitAnyPath(interval time.Duration, timeout time.Duration) error {
	limit := time.Now().Add(timeout)
	devPath := fmt.Sprintf("/dev/disk/by-id/wwn-0x%s", t.DiskID)
	for {
		if time.Now().After(limit) {
			break
		}
		dest, err := os.Readlink(devPath)
		if errors.Is(err, os.ErrNotExist) {
			time.Sleep(interval)
			continue
		}
		if err != nil {
			return err
		}
		if strings.Contains(dest, "/dm-") {
			t.Log().Infof("%s now exists", devPath)
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("timeout waiting for %s to appear", devPath)
}

func (t *T) configureMultipath(ctx context.Context) error {
	realDevPath, err := realpath.Realpath(t.devPath())
	if err != nil {
		return err
	}
	dev := device.New(realDevPath, device.WithLogger(t.Log()))
	return dev.ConfigureMultipath(ctx, 1)
}

func (t *T) configure(ctx context.Context, force forceMode) error {
	exposedDevices := t.ExposedDevices(ctx)
	if force == preserve && len(exposedDevices) > 0 {
		t.Log().Infof("system configuration: skip: device already exposed: %s", exposedDevices)
		return nil
	}
	if t.DiskID == "" {
		return fmt.Errorf("system configuration: disk_id is not set")
	}
	t.Log().Infof("system configuration: scsi scan")
	if err := scsi.LockedScanAll(10 * time.Second); err != nil {
		return fmt.Errorf("system configuration: %w", err)
	}
	if err := t.waitAnyPath(1*time.Second, 30*time.Second); err != nil {
		return err
	}
	udevadm.Settle()
	if err := t.configureMultipath(ctx); err != nil {
		return err
	}
	if err := t.waitDevPath(1*time.Second, 30*time.Second); err != nil {
		return err
	}
	exposedDevices = t.ExposedDevices(ctx)
	if len(exposedDevices) == 0 {
		return fmt.Errorf("system configuration: %s is not exposed device after scan", t.DiskID)
	}
	exposedDevice := exposedDevices[0]
	slaves, err := exposedDevice.Slaves()
	if err != nil {
		return fmt.Errorf("system configuration: %w", err)
	}
	if len(slaves) < 1 {
		return fmt.Errorf("system configuration: no slaves appeared for disk %s", exposedDevice.Path())
	}
	return nil
}
