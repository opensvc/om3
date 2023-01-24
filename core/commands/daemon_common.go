package commands

import (
	"fmt"
	"os"
	"path"
	"time"

	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/file"
)

type (
	CmdDaemonCommon struct{}
)

func (t *CmdDaemonCommon) startDaemon() (err error) {
	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs([]string{"daemon", "start"}),
	)
	_, _ = fmt.Fprintf(os.Stderr, "start daemon\n")
	return cmd.Run()
}

func (t *CmdDaemonCommon) stopDaemon() (err error) {
	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs([]string{"daemon", "stop"}),
	)
	_, _ = fmt.Fprintf(os.Stderr, "Stop daemon\n")
	return cmd.Run()
}

func (t *CmdDaemonCommon) backupLocalConfig(name string) (err error) {
	pathEtc := rawconfig.Paths.Etc
	if !file.ExistsAndDir(pathEtc) {
		_, _ = fmt.Fprintf(os.Stderr, "empty %s, skip backup\n", pathEtc)
		return nil
	}
	cmd := command.New(
		command.WithBufferedStdout(),
		command.WithName(os.Args[0]),
		command.WithArgs([]string{"**", "print", "config", "--local", "--format", "json"}),
		// allow exit code 1 (Error: no match)
		command.WithIgnoredExitCodes(0, 1),
	)
	_, _ = fmt.Fprintln(os.Stderr, "dump all configs")
	if err := cmd.Run(); err != nil {
		return err
	}

	backup := path.Join(pathEtc, name+time.Now().Format(name+"-2006-01-02T15:04:05.json"))
	_, _ = fmt.Fprintf(os.Stderr, "save configs to %s\n", backup)
	if err := os.WriteFile(backup, cmd.Stdout(), 0o400); err != nil {
		return err
	}
	return nil
}

func (t *CmdDaemonCommon) deleteLocalConfig() (err error) {
	pathEtc := rawconfig.Paths.Etc
	if file.ExistsAndDir(pathEtc) {
		cmd := command.New(
			command.WithName(os.Args[0]),
			command.WithArgs([]string{"**", "delete", "--local"}),
		)
		_, _ = fmt.Fprintf(os.Stderr, "delete all config\n")
		if err := cmd.Run(); err != nil {
			return err
		}
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "empty %s, skip delete local config\n", pathEtc)
	}
	return rawconfig.CreateMandatoryDirectories()
}
