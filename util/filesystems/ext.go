package filesystems

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
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
//
// resize2fs writes its banner, and the "Nothing to do!" of a filesystem it
// does not grow, to its stderr: they are logged as what they are, not as
// errors, the exit code saying a failure.
func extGrow(ctx context.Context, log *plog.Logger, dev, _ string) error {
	cmd := command.New(
		command.WithName("resize2fs"),
		command.WithVarArgs(dev),
		command.WithContext(ctx),
		command.WithLogger(log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

// extSizeOnDevice is the size the superblock of the filesystem on dev says:
// its block count by its block size, which is what resize2fs grew it to, and
// can be less than the device, as a device a few blocks short of another
// block group is not taken up.
func extSizeOnDevice(ctx context.Context, dev string) (int64, error) {
	cmd := exec.CommandContext(ctx, "dumpe2fs", "-h", dev)
	b, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("dumpe2fs -h %s: %w", dev, err)
	}
	return parseExtSuperblockSize(string(b))
}

// parseExtSuperblockSize reads the block count and the block size of the
// output of dumpe2fs -h.
func parseExtSuperblockSize(s string) (int64, error) {
	var count, size int64
	for _, line := range strings.Split(s, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "Block count":
			count, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		case "Block size":
			size, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		}
	}
	if count <= 0 || size <= 0 {
		return 0, fmt.Errorf("no block count and block size in the superblock")
	}
	return count * size, nil
}
