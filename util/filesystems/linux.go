//go:build linux

package filesystems

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/file"
)

func (t T) Mount(ctx context.Context, dev string, mnt string, options string) error {
	return t.mount(ctx, dev, mnt, options, nil)
}

// MountNoCanonicalize mounts dev on mnt as Mount does, mnt passed to the
// kernel as it is: mount(8) does not resolve it first. It is for a mnt
// naming a directory the caller opened, as /proc/<pid>/fd/<fd>, which the
// kernel follows to the directory opened, whatever its path leads to by then.
func (t T) MountNoCanonicalize(ctx context.Context, dev string, mnt string, options string) error {
	return t.mount(ctx, dev, mnt, options, []string{"--no-canonicalize"})
}

func (t T) mount(ctx context.Context, dev string, mnt string, options string, extra []string) error {
	args := append([]string{}, extra...)
	if fsType := t.Type(); fsType == "bind" {
		args = append(args, "-o", "bind")
	} else {
		args = append(args, "-t", fsType)
	}
	if len(options) > 0 {
		args = append(args, "-o", options)
	}
	args = append(args, dev, mnt)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("mount"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithTimeout(time.Minute),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	exitCode := cmd.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("%s exit code %d", cmd, exitCode)
	}
	return nil
}

func (t T) Umount(ctx context.Context, mnt string) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("umount"),
		command.WithVarArgs(mnt),
		command.WithLogger(t.Log()),
		command.WithTimeout(time.Minute),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	exitCode := cmd.ExitCode()
	if exitCode == 32 {
		return fmt.Errorf("%s: %w", cmd, syscall.EBUSY)
	} else if exitCode != 0 {
		return fmt.Errorf("%s exit code %d", cmd, exitCode)
	}
	return nil
}

// UmountNoFollow unmounts the mount at mnt, its last component not followed
// if it is a link: mnt names it through a directory the caller opened, as
// /proc/<pid>/fd/<fd>/<name>, and a link swapped in at name is refused
// rather than followed to another mount. It is the umount2 system call,
// umount(8) resolving the path first.
func (t T) UmountNoFollow(ctx context.Context, mnt string) error {
	t.Log().Infof("umount %s", mnt)
	if err := unix.Unmount(mnt, unix.UMOUNT_NOFOLLOW); err != nil {
		return fmt.Errorf("umount %s: %w", mnt, err)
	}
	return nil
}

func (t T) KillUsers(ctx context.Context, mnt string) error {
	var killArgs string
	if v, err := file.ExistsAndDir(mnt); err != nil {
		return err
	} else if v {
		// need to kill users on mount point and sub directories
		killArgs = "-kMmv"
	} else {
		// need to kill users on mount point
		killArgs = "-kMv"
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("fuser"),
		command.WithVarArgs(killArgs, mnt),
		command.WithLogger(t.Log()),
		command.WithTimeout(time.Minute),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	exitCode := cmd.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("%s exit code %d", cmd, exitCode)
	}
	return nil
}
