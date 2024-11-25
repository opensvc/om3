package omcmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdDaemonCommon struct{}
)

// isRunning returns true if daemon is running
func (t *CmdDaemonCommon) isRunning() bool {
	cli, err := client.New()
	if err != nil {
		return false
	}
	if resp, err := cli.GetNodePing(context.Background(), hostname.Hostname()); err != nil {
		return false
	} else if resp.StatusCode != 204 {
		return false
	}
	return true
}

func (t *CmdDaemonCommon) nodeDrain() (err error) {
	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs([]string{"node", "drain", "--wait"}),
	)
	_, _ = fmt.Fprintf(os.Stdout, "Draining node\n")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (t *CmdDaemonCommon) backupLocalConfig(name string) error {
	backupDir := rawconfig.Paths.Backup
	if v, err := file.ExistsAndDir(backupDir); err != nil {
		return err
	} else if !v {
		_, _ = fmt.Fprintf(os.Stdout, "Empty %s, skip backup\n", backupDir)
		return nil
	}
	cmd := command.New(
		command.WithBufferedStdout(),
		command.WithName(os.Args[0]),
		command.WithArgs([]string{"**", "print", "config", "--local", "--format", "json"}),
		// allow exit code 1 (Error: no match)
		command.WithIgnoredExitCodes(0, 1),
	)
	_, _ = fmt.Fprintln(os.Stdout, "Dump all configs")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}

	backup := path.Join(backupDir, time.Now().Format(name+"-2006-01-02T15:04:05.json"))
	_, _ = fmt.Fprintf(os.Stdout, "Save configs to %s\n", backup)
	if err := os.WriteFile(backup, cmd.Stdout(), 0o400); err != nil {
		return err
	}
	return nil
}

func (t *CmdDaemonCommon) deleteLocalConfig() error {
	pathEtc := rawconfig.Paths.Etc
	if v, err := file.ExistsAndDir(pathEtc); err != nil {
		return err
	} else if v {
		cmd := command.New(
			command.WithName(os.Args[0]),
			command.WithArgs([]string{"**", "delete", "--local"}),
		)
		_, _ = fmt.Fprintf(os.Stdout, "Delete all config\n")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "Empty %s, skip delete local config\n", pathEtc)
	}
	return rawconfig.CreateMandatoryDirectories()
}
