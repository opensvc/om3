//go:build !linux

package filesystems

import (
	"context"
	"fmt"
)

func (t T) Mount(ctx context.Context, dev string, mnt string, options string) error {
	return fmt.Errorf("mount not implemented")
}

func (t T) Umount(ctx context.Context, mnt string) error {
	return nil
}

func (t T) KillUsers(ctx context.Context, mnt string) error {
	return nil
}
