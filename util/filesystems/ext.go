package filesystems

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/findmnt"
	"github.com/opensvc/om3/v3/util/plog"
)

func extCanFSCK() error {
	if _, err := exec.LookPath("e2fsck"); err != nil {
		return err
	}
	return nil
}

func extFSCK(ctx context.Context, s string) error {
	cmd := exec.CommandContext(ctx, "e2fsck", "-p", s)
	cmd.Start()
	cmd.Wait()
	exitCode := cmd.ProcessState.ExitCode()
	switch exitCode {
	case 0: // All good
		return nil
	case 1: // File system errors corrected
		return nil
	case 32: // E2fsck canceled by user request
		return nil
	case 33: // ?
		return nil
	default:
		return fmt.Errorf("%s exit code: %d", cmd, exitCode)
	}
}

func extIsFormated(ctx context.Context, s string) (bool, error) {
	if _, err := exec.LookPath("tune2fs"); err != nil {
		return false, errors.New("tune2fs not found")
	}
	cmd := exec.CommandContext(ctx, "tune2fs", "-l", s)
	cmd.Start()
	cmd.Wait()
	exitCode := cmd.ProcessState.ExitCode()
	switch exitCode {
	case 0: // All good
		return true, nil
	default:
		return false, nil
	}
}

func xMKFS(ctx context.Context, x string, s string, xargs []string, log *plog.Logger) error {
	if _, err := exec.LookPath(x); err != nil {
		return fmt.Errorf("%s not found", x)
	}
	args := []string{"-F", "-q", s}
	args = append(args, xargs...)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(x),
		command.WithArgs(args),
		command.WithLogger(log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

// Grow takes the filesystem up to the size of the device it sits on. resize2fs
// does that online.
func extGrow(ctx context.Context, log *plog.Logger, dev, _ string) error {
	cmd := command.New(
		command.WithName("resize2fs"),
		command.WithVarArgs(dev),
		command.WithContext(ctx),
		command.WithLogger(log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

// extCanShrink says whether the filesystem could be shrunk now.
//
// resize2fs only shrinks a filesystem that is not mounted, and it is asked
// before the device under it is touched: a refusal after the device has
// shrunk is data loss.
func extCanShrink(ctx context.Context, dev, mountPoint string) error {
	if _, err := exec.LookPath("resize2fs"); err != nil {
		return fmt.Errorf("resize2fs is not installed")
	}
	if mountPoint == "" {
		return nil
	}
	if mounts, err := findmnt.List(ctx, dev, mountPoint); err != nil {
		return err
	} else if len(mounts) > 0 {
		return fmt.Errorf("an ext filesystem shrinks only while it is unmounted, and %s is mounted on %s", dev, mountPoint)
	}
	return nil
}

func extShrink(ctx context.Context, log *plog.Logger, dev, mountPoint string, size int64) error {
	if err := extCanShrink(ctx, dev, mountPoint); err != nil {
		return err
	}
	// e2fsck is mandatory before a shrink, and resize2fs refuses without it.
	if err := extFSCK(ctx, dev); err != nil {
		return fmt.Errorf("check before shrink: %w", err)
	}
	cmd := command.New(
		command.WithName("resize2fs"),
		command.WithVarArgs(dev, fmt.Sprintf("%ds", size/512)),
		command.WithContext(ctx),
		command.WithLogger(log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}
