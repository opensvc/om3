package resfshost

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/filesystems"
)

// wipeBackupDir is where the unprovision of the resource keeps the
// signatures it wipes from its device, for an admin to restore them: out of
// the directories of the object, so a purge leaves them, until the next
// provision of the resource.
func (t *T) wipeBackupDir() string {
	return filepath.Join(rawconfig.Paths.Var, "backup", "wipefs", t.Path.Namespace, t.Path.Kind.String(), t.Path.Name, t.RID())
}

// wipeBackupName is the name wipefs gives the backup of a signature, which
// says its offset on the device.
var wipeBackupName = regexp.MustCompile(`^wipefs-.+-0x([0-9a-fA-F]+)\.bak$`)

// wipeSignatures erases the signatures of the device the provision of the
// resource formatted, as v2 did not: the next provision found the signature
// of the filesystem it made, and refused to format it.
//
// It wipes only what the provision of the resource made on this node, on a
// block device of its own: a provision not recorded did not make it, a
// shared device is the one of every node, a btrfs subvolume shares its
// device with others, and a file or a network filesystem is no device. The
// signatures are kept in wipeBackupDir, and the log says how to restore them.
func (t *T) wipeSignatures(ctx context.Context) error {
	if !resource.ProvisionRecorded(t) {
		return nil
	}
	fs := t.fs()
	if fs.IsVirtual() || fs.IsNetworked() || fs.IsFileBacked() || t.isBind() || t.hasMountOption("loop") {
		return nil
	}
	if _, ok := fs.(filesystems.MKFSer); !ok {
		return nil
	}
	devpath := t.devpath(ctx)
	switch {
	case t.IsShared():
		t.Log().Infof("skip the wipe of the signatures of %s: the device is shared", t.Device)
		return nil
	case t.isBtrfs() && strings.Contains(t.mountOptions(), "subvol="):
		t.Log().Infof("skip the wipe of the signatures of %s: a btrfs subvolume shares its device", t.Device)
		return nil
	case !strings.HasPrefix(devpath, "/dev/"):
		t.Log().Infof("skip the wipe of the signatures of %s: not a device", t.Device)
		return nil
	case !file.Exists(devpath):
		t.Log().Infof("skip the wipe of the signatures of %s: %s does not exist", t.Device, devpath)
		return nil
	}
	dir := t.wipeBackupDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	before := wipeBackups(dir)
	// wipefs writes the backups to the home directory, and only recent
	// versions take another: the home of the command is the backup
	// directory.
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("wipefs"),
		command.WithVarArgs("--all", "--backup", devpath),
		command.WithEnv([]string{"HOME=" + dir}),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wipe the signatures of %s: %w", devpath, err)
	}
	after := wipeBackups(dir)
	names := make([]string, 0, len(after))
	for name := range after {
		if !before[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		p := filepath.Join(dir, name)
		if m := wipeBackupName.FindStringSubmatch(name); m != nil {
			offset, _ := strconv.ParseInt(m[1], 16, 64)
			t.Log().Infof("wiped a signature of %s, kept in %s: restore it with: dd if=%s of=%s seek=%d bs=1 conv=notrunc", devpath, p, p, devpath, offset)
		} else {
			t.Log().Infof("wiped a signature of %s, kept in %s", devpath, p)
		}
	}
	if err := t.device(ctx).FlushBufs(ctx); err != nil {
		t.Log().Warnf("%s", err)
	}
	return nil
}

// wipeBackups returns the names of the backups of the directory.
func wipeBackups(dir string) map[string]bool {
	m := make(map[string]bool)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() {
			m[e.Name()] = true
		}
	}
	return m
}

// removeWipeBackups removes the signatures an unprovision of the resource
// kept, once a provision made it again: they are of a filesystem gone.
func (t *T) removeWipeBackups() {
	dir := t.wipeBackupDir()
	if !file.Exists(dir) {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Log().Warnf("remove the signatures the unprovision kept in %s: %s", dir, err)
		return
	}
	t.Log().Infof("removed the signatures the unprovision kept in %s", dir)
}
