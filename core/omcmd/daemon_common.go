package omcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/mountinfo"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	CmdDaemonCommon struct {
		log *plog.Logger
	}
)

// setLogger sets the logger of the command steps, its messages headed by
// prefix to tell the command they are steps of. The daemon forks the cluster
// join and leave commands, with no one to read their output: their steps are
// logged, which reaches the journal, and the console when there is one. It
// is called by the command Run, once the logging is configured.
func (t *CmdDaemonCommon) setLogger(prefix string) {
	t.log = plog.NewDefaultLogger().Attr("pkg", "core/omcmd").WithPrefix(prefix)
}

// logger returns the logger of the command steps, set by setLogger.
func (t *CmdDaemonCommon) logger() *plog.Logger {
	if t.log == nil {
		t.setLogger("")
	}
	return t.log
}

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
	t.logger().Infof("Draining node")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

// backupLocalConfig moves the config directory aside, under the backup
// directory, in a directory named after name and the time. It returns the
// path of the backup, empty when the backup directory is missing and the
// config is left as it is.
func (t *CmdDaemonCommon) backupLocalConfig(name string) (string, error) {
	backupDir := rawconfig.Paths.Backup
	if v, err := file.ExistsAndDir(backupDir); err != nil {
		return "", err
	} else if !v {
		t.logger().Infof("Missing directory %s, skip backup", backupDir)
		return "", nil
	}

	backup := path.Join(backupDir, time.Now().Format(name+"-2006-01-02T15:04:05"))
	t.logger().Infof("move all configs to %s", backup)
	// Prefer file.Move to avoid syscall.EXDEV errors on os.Rename
	err := file.Move(rawconfig.Paths.Etc, backup)
	if err != nil {
		return "", err
	}
	return backup, nil
}

// dropBackup removes the config backup made by backupLocalConfig, once the
// command it was the rollback point of has succeeded. It holds the secrets
// of the cluster the node was a member of, which the node has no use of
// anymore. The command has succeeded: a failure is warned.
func (t *CmdDaemonCommon) dropBackup(backup string) {
	if backup == "" {
		return
	}
	t.logger().Infof("Drop the config backup %s", backup)
	if err := os.RemoveAll(backup); err != nil {
		t.logger().Warnf("drop the config backup: %s", err)
	}
}

// cleanupVarDir removes the entries of the opensvc var directory, but the
// backup directory and the filesystems mounted under it, as the one of a
// pool: they are the node's, not the cluster's, and what they hold is not
// removed. An entry that can not be removed does not stop the removal of the
// others, so the node left with a daemon to start has as little as possible
// of the cluster it left.
func (t *CmdDaemonCommon) cleanupVarDir() error {
	varDir, err := filepath.EvalSymlinks(rawconfig.Paths.Var)
	if err != nil {
		return err
	}
	backup := filepath.Join(varDir, filepath.Base(rawconfig.Paths.Backup))
	mounts, err := mountsUnder(varDir)
	if err != nil {
		return err
	}
	return removeTree(t.logger(), varDir, append(mounts, backup))
}

// mountsUnder returns the mount points under dir.
func mountsUnder(dir string) ([]string, error) {
	entries, err := mountinfo.Read()
	if err != nil {
		return nil, fmt.Errorf("read the mounts: %w", err)
	}
	l := make([]string, 0)
	for _, e := range entries {
		if strings.HasPrefix(e.Target, dir+"/") {
			l = append(l, e.Target)
		}
	}
	return l, nil
}

// removeTree removes the entries of dir, but the paths of keep and what they
// hold. A directory holding a path of keep is not removed, its other entries
// are.
func removeTree(log *plog.Logger, dir string, keep []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", dir, err)
	}
	var errs []error
	for _, entry := range entries {
		p := filepath.Join(dir, entry.Name())
		if slices.Contains(keep, p) {
			log.Infof("Keep %s", p)
			continue
		}
		holds := slices.ContainsFunc(keep, func(s string) bool {
			return strings.HasPrefix(s, p+"/")
		})
		if holds && entry.IsDir() {
			if err := removeTree(log, p, keep); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, fmt.Errorf("remove %s: %w", p, err))
		}
	}
	return errors.Join(errs...)
}

// keptConfig is what the node keeps of its config when it joins or leaves a
// cluster, read before the config is moved aside and written back after: the
// options of the [collector] section of the cluster config, and of the [node]
// section of the node config, as written.
type keptConfig struct {
	collector map[string]string
	node      map[string]string
}

// readNodeKeys returns the options of the [node] section of the node config
// the node has among options, as written.
func readNodeKeys(options []string) map[string]string {
	kv := make(map[string]string)
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return kv
	}
	section := n.Config().SectionMap("node")
	for _, option := range options {
		if v, ok := section[option]; ok {
			kv[option] = v
		}
	}
	return kv
}

// write writes the [collector] section in the cluster config, which the
// daemon bootstraps keeping the keys it does not set, and the kept node keys
// in the node config. Each is written whether the other fails or not.
func (t keptConfig) write() error {
	var errs []error
	if len(t.collector) > 0 {
		if ccfg, err := object.NewCluster(object.WithVolatile(false)); err != nil {
			errs = append(errs, fmt.Errorf("keep the collector settings: %w", err))
		} else if err := setKeys(ccfg.Config(), "collector", t.collector); err != nil {
			errs = append(errs, fmt.Errorf("keep the collector settings: %w", err))
		}
	}
	if len(t.node) > 0 {
		if n, err := object.NewNode(object.WithVolatile(false)); err != nil {
			errs = append(errs, fmt.Errorf("keep the node keys: %w", err))
		} else if err := setKeys(n.Config(), "node", t.node); err != nil {
			errs = append(errs, fmt.Errorf("keep the node keys: %w", err))
		}
	}
	return errors.Join(errs...)
}

// setKeys sets the options of kv in section of cfg.
func setKeys(cfg *xconfig.T, section string, kv map[string]string) error {
	options := make([]string, 0, len(kv))
	for option := range kv {
		options = append(options, option)
	}
	sort.Strings(options)
	ops := make([]keyop.T, 0, len(options))
	for _, option := range options {
		ops = append(ops, *keyop.New(key.New(section, option), keyop.Set, kv[option], 0))
	}
	return cfg.Set(ops...)
}
