//go:build !linux

package filesystems

import "fmt"

func (t T) Mount(dev string, mnt string, options string) error {
	return fmt.Errorf("mount not implemented")
}

func (t T) Umount(mnt string) error {
	return nil
}

func (t T) KillUsers(mnt string) error {
	return nil
}
