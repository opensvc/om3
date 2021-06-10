package filesystem

import (
	"errors"
	"fmt"
	"os/exec"
)

var (
	T_XFS T = T{
		name:       "xfs",
		mkfs:       xfsMKFS,
		isFormated: xfsIsFormated,
	}
)

func xfsIsFormated(s string) (bool, error) {
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

func xfsMKFS(s string) error {
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
