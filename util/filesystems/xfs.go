package filesystems

import (
	"errors"
	"fmt"
	"os/exec"
)

type (
	T_XFS struct{ T }
)

func init() {
	registerFS(NewXFS())
}

func NewXFS() *T_XFS {
	t := T_XFS{
		T{fsType: "xfs"},
	}
	return &t
}

func (t T) IsFormated(s string) (bool, error) {
	if _, err := exec.LookPath("xfs_admin"); err != nil {
		return false, errors.New("xfs_admin not found")
	}
	cmd := exec.Command("xfs_admin", "-l", s)
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

func (t T_XFS) MKFS(s string) error {
	if _, err := exec.LookPath("mkfs.xfs"); err != nil {
		return fmt.Errorf("mkfs.xfs not found")
	}
	cmd := exec.Command("mkfs.xfs", "-f", "-q", s)
	cmd.Start()
	cmd.Wait()
	exitCode := cmd.ProcessState.ExitCode()
	switch exitCode {
	case 0: // All good
		return nil
	default:
		return fmt.Errorf("%s exit code %d", cmd, exitCode)
	}
}
