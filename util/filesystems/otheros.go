//go:build !linux

package filesystems

import (
	"context"
	"fmt"
)

func (t T) Mount(ctx context.Context, dev string, mnt string, options string) error {
	return fmt.Errorf("mount not implemented")
}

func (t T) MountNoCanonicalize(ctx context.Context, dev string, mnt string, options string) error {
	return fmt.Errorf("mount not implemented")
}

func (t T) Umount(ctx context.Context, mnt string) error {
	return nil
}

func (t T) UmountNoFollow(ctx context.Context, mnt string) error {
	return fmt.Errorf("umount not implemented")
}

func (t T) KillUsers(ctx context.Context, mnt string) error {
	return nil
}
