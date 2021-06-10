package filesystem

import (
	"errors"
	"fmt"
	"os/exec"
)

var (
	T_Ext2 T = T{
		name:       "ext2",
		fsck:       extFSCK,
		canFSCK:    extCanFSCK,
		mkfs:       ext2MKFS,
		isFormated: extIsFormated,
	}
	T_Ext3 T = T{
		name:       "ext3",
		fsck:       extFSCK,
		canFSCK:    extCanFSCK,
		mkfs:       ext3MKFS,
		isFormated: extIsFormated,
	}
	T_Ext4 T = T{
		name:       "ext4",
		fsck:       extFSCK,
		canFSCK:    extCanFSCK,
		mkfs:       ext4MKFS,
		isFormated: extIsFormated,
	}
)

func extCanFSCK() error {
	if _, err := exec.LookPath("e2fsck"); err != nil {
		return err
	}
	return nil
}

func extFSCK(s string) error {
	cmd := exec.Command("e2fsck", "-p", s)
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

func extIsFormated(s string) (bool, error) {
	if _, err := exec.LookPath("tune2fs"); err != nil {
		return false, errors.New("tune2fs not found")
	}
	cmd := exec.Command("tune2fs", "-l", s)
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

func ext2MKFS(s string) error {
	return xMKFS("mkfs.ext2", s)
}

func ext3MKFS(s string) error {
	return xMKFS("mkfs.ext3", s)
}

func ext4MKFS(s string) error {
	return xMKFS("mkfs.ext3", s)
}

func xMKFS(x string, s string) error {
	if _, err := exec.LookPath(x); err != nil {
		return fmt.Errorf("%s not found", x)
	}
	cmd := exec.Command(x, "-F", "-q", s)
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
