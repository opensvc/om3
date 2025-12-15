package filesystems

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
)

type (
	XFS struct{ T }
)

func init() {
	registerFS(NewXFS())
}

func NewXFS() *XFS {
	t := XFS{
		T{fsType: "xfs"},
	}
	return &t
}

func (t XFS) IsFormated(ctx context.Context, s string) (bool, error) {
	if _, err := exec.LookPath("xfs_admin"); err != nil {
		return false, errors.New("xfs_admin not found")
	}
	cmd := exec.CommandContext(ctx, "xfs_admin", "-l", s)
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

func (t XFS) MKFS(ctx context.Context, devpath string, args []string) error {
	if _, err := exec.LookPath("mkfs.xfs"); err != nil {
		return fmt.Errorf("mkfs.xfs not found")
	}
	cmd := command.New(
		command.WithName("mkfs.xfs"),
		command.WithArgs(append(args, "-f", "-q", devpath)),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t XFS) IsCapable() bool {
	if _, err := exec.LookPath("mkfs.xfs"); err != nil {
		return false
	}
	return true
}
