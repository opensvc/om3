// +build linux

package filesystems

import (
	"fmt"
	"os/exec"
)

func (t T) Mount(dev string, mnt string, options string) error {
	/*
		cmd := xexec.NewVerboseCmd(
			"mount",
			xexec.WithArgs("-t", t.Type(), "-o", options, dev, mnt, ...),
			xexec.WithLogger(t.Log().Str("foo", "bar")),
		)
		cmd.Run()
		exitCode := cmd.ExitCode()
	*/
	cmd := exec.Command("mount", "-t", t.Type(), "-o", options, dev, mnt)
	cmd.Start()
	cmd.Wait()
	exitCode := cmd.ProcessState.ExitCode()
	if cmd.ProcessState.ExitCode() != 0 {
		return fmt.Errorf("%s exit code %d", cmd, exitCode)
	}
	return nil
}

func (t T) Umount(mnt string) error {
	cmd := exec.Command("umount", mnt)
	cmd.Start()
	cmd.Wait()
	exitCode := cmd.ProcessState.ExitCode()
	if cmd.ProcessState.ExitCode() != 0 {
		return fmt.Errorf("%s exit code %d", cmd, exitCode)
	}
	return nil
}
