package omcmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/hostname"
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

	backup := path.Join(backupDir, time.Now().Format(name+"-2006-01-02T15:04:05"))
	_, _ = fmt.Fprintf(os.Stdout, "move all configs to %s\n", backup)
	err := os.Rename(rawconfig.Paths.Etc, backup)
	if err != nil {
		return err
	}
	return nil
}

func (t *CmdDaemonCommon) cleanupAndMandatoryDirectories() error {
	if err := t.cleanupVarDir(); err != nil {
		return fmt.Errorf("cleanup opensvc var dir: %w", err)
	}

	return rawconfig.CreateMandatoryDirectories()
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
