//go:build linux || solaris || freebsd || darwin

package device

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

func (t T) Stat() (unix.Stat_t, error) {
	stat := unix.Stat_t{}
	err := unix.Stat(t.path, &stat)
	return stat, err
}

func (t T) MajorMinorStr() (string, error) {
	major, minor, err := t.MajorMinor()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", major, minor), nil
}

func (t T) MajorMinor() (uint32, uint32, error) {
	stat, err := t.Stat()
	if err != nil {
		return 0, 0, err
	}
	return unix.Major(uint64(stat.Rdev)), unix.Minor(uint64(stat.Rdev)), nil
}

func (t T) Major() (uint32, error) {
	stat, err := t.Stat()
	if err != nil {
		return 0, err
	}
	return unix.Major(uint64(stat.Rdev)), nil
}

func (t T) Minor() (uint32, error) {
	stat, err := t.Stat()
	if err != nil {
		return 0, err
	}
	return unix.Minor(uint64(stat.Rdev)), nil
}

func (t T) MknodBlock(major, minor uint32) error {
	return t.mknod(syscall.S_IFBLK, major, minor)
}

func (t T) mknod(mode, major, minor uint32) error {
	if err := os.MkdirAll(filepath.Dir(t.path), 644); err != nil {
		return fmt.Errorf("failed to create directory: %s", err)
	}

	if err := unix.Mknod(t.path, mode|uint32(os.FileMode(0660)), int(unix.Mkdev(major, minor))); err != nil {
		return fmt.Errorf("failed to create device %s: %s", t.path, err)
	}
	return nil
}
