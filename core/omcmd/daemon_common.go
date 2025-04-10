package omcmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
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

func (t *CmdDaemonCommon) nodeDrain(ctx context.Context) (err error) {
	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs([]string{"node", "drain", "--wait"}),
		command.WithContext(ctx),
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
	if err := t.cleanupEtcDir(); err != nil {
		return fmt.Errorf("cleanup opensvc etc dir: %w", err)
	}

	if err := t.cleanupVarDir(); err != nil {
		return fmt.Errorf("cleanup opensvc var dir: %w", err)
	}

	return rawconfig.CreateMandatoryDirectories()
}

func (t *CmdDaemonCommon) cleanupEtcDir() error {
	etcDir := rawconfig.Paths.Etc
	if ok, err := file.ExistsAndDir(etcDir); err != nil {
		return err
	} else if ok {
		return os.RemoveAll(etcDir)
	}
	return nil
}

// cleanupVarDir removes all entries in the opensvc var directory except for
// the backup directory.
func (t *CmdDaemonCommon) cleanupVarDir() error {
	varDir := rawconfig.Paths.Var
	backupName := filepath.Base(rawconfig.Paths.Backup)

	files, err := os.ReadDir(varDir)
	if err != nil {
		return fmt.Errorf("unable to read base directory: %w", err)
	}

	for _, file := range files {
		if file.Name() == backupName {
			continue
		}
		path := filepath.Join(varDir, file.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("unable to remove path %s: %w", path, err)
		}
	}
	return nil
}
