// +build linux

package resdiskraw

import (
	"path/filepath"
	"strings"

	"github.com/yookoala/realpath"

	"opensvc.com/opensvc/util/device"
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

func (t T) ExposedDevices() []*device.T {
	l := make([]*device.T, 0)
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
