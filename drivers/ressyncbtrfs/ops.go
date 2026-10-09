package ressyncbtrfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/btrfs"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	// execOps runs the btrfs commands of a sync, on the local node or over
	// ssh.
	//
	// The snapshots are kept outside the subvolumes synced, in the root of
	// the filesystem, which a sync mounts at a mount point of its own, on
	// every node it works on, and unmounts at its end: a mount of the root
	// left behind would keep the device busy, and the stop of the service
	// on a node it was left on would fail to release the device.
	execOps struct {
		t *T

		mu sync.Mutex
		// roots are the mount points of the roots of the filesystems, by
		// node and label.
		roots map[rootKey]string
	}

	rootKey struct {
		nodename string
		label    string
	}
)

// isLocal reports whether nodename names the local node, "" included.
func (t *T) isLocal(nodename string) bool {
	return nodename == "" || nodename == hostname.Hostname()
}

// shQuote quotes s for a posix shell.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// mountDirName is the name the directory the root of the filesystem labeled
// label is mounted on starts with. A label is any text, a '/' and a ".."
// included, which a path component is not: what else it holds is replaced,
// the mount directory staying in the btrfs directory of the agent.
func mountDirName(label string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, label)
}

// run runs the shell script on nodename, over the connection of the run to
// it, and returns its stdout.
func (t *T) run(nodename, script string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	var err error
	if t.isLocal(nodename) {
		cmd := exec.Command("/bin/sh", "-c", script)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
	} else {
		var session *ssh.Session
		if session, err = t.newSession(nodename); err != nil {
			return nil, err
		}
		defer session.Close()
		session.Stdout = &stdout
		session.Stderr = &stderr
		err = session.Run("/bin/sh -c " + shQuote(script))
	}
	if err != nil {
		where := nodename
		if t.isLocal(nodename) {
			where = "localhost"
		}
		return nil, fmt.Errorf("%s: %w: %s", where, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// root mounts the root of the filesystem of label on nodename, at a mount
// point of its own, once per sync, and returns the mount point.
func (o *execOps) root(nodename, label string) (string, error) {
	if o.t.isLocal(nodename) {
		nodename = ""
	}
	key := rootKey{nodename: nodename, label: label}
	o.mu.Lock()
	defer o.mu.Unlock()
	if p, ok := o.roots[key]; ok {
		return p, nil
	}
	dir := filepath.Join(rawconfig.Paths.Var, "btrfs")
	// The device is found by probing, as a loop device has no
	// /dev/disk/by-label link for "mount LABEL=", nor "blkid -l", to find
	// it by, and the devices of a btrfs spanning several are scanned for
	// the mount to assemble them.
	script := fmt.Sprintf(`set -e
d=$(blkid -o device -t %[3]s | head -n 1)
if [ -z "$d" ]; then echo "no device labeled "%[2]s >&2; exit 1; fi
btrfs device scan >/dev/null 2>&1 || true
mkdir -p %[1]s
p=$(mktemp -d %[4]s)
if ! mount -t btrfs -o subvolid=5 "$d" "$p"; then rmdir "$p"; exit 1; fi
echo "$p"`, shQuote(dir), shQuote(label), shQuote("LABEL="+label), shQuote(filepath.Join(dir, mountDirName(label)+".XXXXXX")))
	b, err := o.t.run(nodename, script)
	if err != nil {
		return "", fmt.Errorf("mount the root of the btrfs filesystem labeled %s: %w", label, err)
	}
	p := strings.TrimSpace(string(b))
	if !strings.HasPrefix(p, dir+"/") {
		return "", fmt.Errorf("mount the root of the btrfs filesystem labeled %s: unexpected mount point %q", label, p)
	}
	if o.roots == nil {
		o.roots = make(map[rootKey]string)
	}
	o.roots[key] = p
	return p, nil
}

// release unmounts the roots the sync mounted.
func (o *execOps) release() {
	o.mu.Lock()
	defer o.mu.Unlock()
	for key, p := range o.roots {
		if _, err := o.t.run(key.nodename, fmt.Sprintf("umount %[1]s && rmdir %[1]s", shQuote(p))); err != nil {
			o.t.Log().Warnf("unmount the root of the btrfs filesystem labeled %s: %s", key.label, err)
		}
		delete(o.roots, key)
	}
}

// list lists the subvolumes of the filesystem of label on nodename, the
// read-only ones alone when readOnly.
func (o *execOps) list(nodename, label string, readOnly bool) (string, []btrfs.Subvol, error) {
	root, err := o.root(nodename, label)
	if err != nil {
		return "", nil, err
	}
	args := btrfs.ListArgs(readOnly, root)
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shQuote(arg)
	}
	b, err := o.t.run(nodename, "btrfs "+strings.Join(quoted, " "))
	if err != nil {
		return "", nil, err
	}
	l, err := btrfs.ParseList(b)
	return root, l, err
}

func (o *execOps) listRuns(nodename, label, dir string) ([]run, error) {
	_, l, err := o.list(nodename, label, true)
	if err != nil {
		return nil, err
	}
	return runsOf(l, dir)
}

// runsOf groups the read-only snapshots in the run directories of dir by
// run. A snapshot a receive was interrupted in is not read-only, and is not
// one of the run.
func runsOf(l []btrfs.Subvol, dir string) ([]run, error) {
	byName := make(map[string]*run)
	for _, s := range l {
		rest, ok := strings.CutPrefix(s.Path, dir+"/")
		if !ok {
			continue
		}
		name, snapPart, ok := strings.Cut(rest, "/")
		if !ok || strings.Contains(snapPart, "/") {
			continue
		}
		// A run is named after the time it was taken, and is sent to,
		// installed from and deleted in the directory of that name: a
		// directory named otherwise is no run of the sync.
		if _, err := time.Parse(runNameFormat, name); err != nil {
			continue
		}
		rel, err := relOfSnapName(snapPart)
		if err != nil {
			continue
		}
		r, ok := byName[name]
		if !ok {
			r = &run{Name: name, Snaps: make(map[string]snap)}
			byName[name] = r
		}
		r.Snaps[rel] = snap{Rel: rel, Path: s.Path, ID: s.Identity(), CGen: s.CGen}
	}
	runs := make([]run, 0, len(byName))
	for _, r := range byName {
		runs = append(runs, *r)
	}
	sortRuns(runs)
	return runs, nil
}

func (o *execOps) nestedSubvols(label, head string) ([]string, error) {
	_, all, err := o.list("", label, false)
	if err != nil {
		return nil, err
	}
	_, ro, err := o.list("", label, true)
	if err != nil {
		return nil, err
	}
	return nestedOf(all, ro, head), nil
}

// nestedOf are the paths from head of the writable subvolumes nested in it.
func nestedOf(all, ro []btrfs.Subvol, head string) []string {
	readOnly := make(map[int64]bool, len(ro))
	for _, s := range ro {
		readOnly[s.ID] = true
	}
	l := make([]string, 0)
	for _, s := range all {
		rel, ok := strings.CutPrefix(s.Path, head+"/")
		if !ok || readOnly[s.ID] {
			continue
		}
		l = append(l, rel)
	}
	sortRels(l)
	return l
}

func (o *execOps) takeRun(label, head string, rels []string, dir string) (run, error) {
	root, err := o.root("", label)
	if err != nil {
		return run{}, err
	}
	all := append([]string{""}, rels...)
	lines := []string{"set -e", "mkdir -p " + shQuote(filepath.Join(root, dir))}
	for _, rel := range all {
		src := filepath.Join(root, head, rel)
		dst := filepath.Join(root, dir, snapName(rel))
		lines = append(lines, "btrfs subvolume snapshot -r "+shQuote(src)+" "+shQuote(dst))
	}
	o.t.Log().Infof("snapshot %s in %s", strings.Join(append([]string{head}, prefixed(head, rels)...), " "), dir)
	if _, err := o.t.run("", strings.Join(lines, "\n")); err != nil {
		return run{}, err
	}
	runs, err := o.listRuns("", label, filepath.Dir(dir))
	if err != nil {
		return run{}, err
	}
	name := filepath.Base(dir)
	for _, r := range runs {
		if r.Name == name {
			return r, nil
		}
	}
	return run{}, fmt.Errorf("the snapshots taken in %s are not listed", dir)
}

func prefixed(head string, rels []string) []string {
	l := make([]string, len(rels))
	for i, rel := range rels {
		l[i] = filepath.Join(head, rel)
	}
	return l
}

func (o *execOps) send(ctx context.Context, nodename, srcLabel, path, parent, dstLabel, dir string) error {
	srcRoot, err := o.root("", srcLabel)
	if err != nil {
		return err
	}
	dstRoot, err := o.root(nodename, dstLabel)
	if err != nil {
		return err
	}
	args := []string{"btrfs", "send"}
	if parent != "" {
		args = append(args, "-p", filepath.Join(srcRoot, parent))
	}
	args = append(args, filepath.Join(srcRoot, path))
	receive := fmt.Sprintf("mkdir -p %[1]s && btrfs receive %[1]s", shQuote(filepath.Join(dstRoot, dir)))
	return o.t.pipe(ctx, nodename, args, receive)
}

// pipe pipes the stdout of the local command args to the shell script
// receive run on nodename.
func (t *T) pipe(ctx context.Context, nodename string, args []string, receive string) error {
	nfoWriter := t.Log().Writer(zerolog.InfoLevel)
	errWriter := &progressWriter{info: nfoWriter, err: t.Log().Writer(zerolog.ErrorLevel)}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stderr = nfoWriter
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stats := ressync.NewStats(nodename)
	if t.isLocal(nodename) {
		rcmd := exec.CommandContext(ctx, "/bin/sh", "-c", receive)
		rcmd.Stdout = nfoWriter
		rcmd.Stderr = errWriter
		stdin, err := rcmd.StdinPipe()
		if err != nil {
			return err
		}
		t.Log().Infof("%s | %s", cmd, receive)
		if err := rcmd.Start(); err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			_ = stdin.Close()
			_ = rcmd.Wait()
			return err
		}
		_, copyErr := t.CopyWithStats(ctx, stdin, stdout, stats)
		_ = stdin.Close()
		return errors.Join(copyErr, waitErr(cmd.String(), cmd.Wait()), waitErr(receive, rcmd.Wait()))
	}
	session, err := t.newSession(nodename)
	if err != nil {
		return err
	}
	defer session.Close()
	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}
	session.Stdout = nfoWriter
	session.Stderr = errWriter
	t.Log().Infof("%s | ssh %s %s", cmd, nodename, receive)
	if err := session.Start("/bin/sh -c " + shQuote(receive)); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = session.Wait()
		return err
	}
	_, copyErr := t.CopyWithStats(ctx, stdin, stdout, stats)
	_ = stdin.Close()
	return errors.Join(copyErr, waitErr(cmd.String(), cmd.Wait()), waitErr("ssh "+nodename+" "+receive, session.Wait()))
}

// progressWriter logs the lines btrfs send and receive write on stderr: the
// progress lines, "At subvol <name>" and "At snapshot <name>", as
// information, and the others as errors.
type progressWriter struct {
	info, err io.Writer
	buf       []byte
}

func (w *progressWriter) Write(b []byte) (int, error) {
	w.buf = append(w.buf, b...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(b), nil
		}
		line := w.buf[:i+1]
		out := w.err
		if bytes.HasPrefix(line, []byte("At subvol ")) || bytes.HasPrefix(line, []byte("At snapshot ")) {
			out = w.info
		}
		if _, err := out.Write(line); err != nil {
			return len(b), err
		}
		w.buf = w.buf[i+1:]
	}
}

func waitErr(what string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", what, err)
}

// deleteOrder is the subvolumes of l at path and below it, the deepest
// first, which is the order they can be deleted in.
func deleteOrder(l []btrfs.Subvol, path string) []string {
	paths := make([]string, 0)
	for _, s := range btrfs.Under(l, path) {
		paths = append(paths, s.Path)
	}
	slices.SortFunc(paths, func(a, b string) int {
		if d := strings.Count(b, "/") - strings.Count(a, "/"); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	return paths
}

func deleteLine(root string, paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = shQuote(filepath.Join(root, p))
	}
	return "btrfs subvolume delete " + strings.Join(quoted, " ")
}

func (o *execOps) installHead(nodename, label, dir string, r run, head string) error {
	b, err := o.t.run(nodename, "cat /proc/self/mountinfo")
	if err != nil {
		return err
	}
	if mnt, ok := btrfs.MountOfSubvol(b, head, true); ok {
		return fmt.Errorf("subvolume %s is mounted on %s: the service runs there, and its data is not replaced", head, mnt)
	}
	root, all, err := o.list(nodename, label, false)
	if err != nil {
		return err
	}
	staged := head + ".osync-new"
	lines := []string{"set -e"}
	if line := deleteLine(root, deleteOrder(all, staged)); line != "" {
		lines = append(lines, line)
	}
	stagedPath := filepath.Join(root, staged)
	lines = append(lines, "btrfs subvolume snapshot "+shQuote(filepath.Join(root, dir, snapName("")))+" "+shQuote(stagedPath))
	for _, rel := range r.rels() {
		if rel == "" {
			continue
		}
		// The snapshot of the head holds an empty directory where a
		// subvolume was nested, which the snapshot of that subvolume
		// replaces.
		p := filepath.Join(stagedPath, rel)
		lines = append(lines,
			"if [ -d "+shQuote(p)+" ]; then rmdir "+shQuote(p)+"; fi",
			"mkdir -p "+shQuote(filepath.Dir(p)),
			"btrfs subvolume snapshot "+shQuote(filepath.Join(root, dir, snapName(rel)))+" "+shQuote(p),
		)
	}
	if line := deleteLine(root, deleteOrder(all, head)); line != "" {
		lines = append(lines, line)
	}
	headPath := filepath.Join(root, head)
	lines = append(lines,
		"mkdir -p "+shQuote(filepath.Dir(headPath)),
		"mv "+shQuote(stagedPath)+" "+shQuote(headPath),
	)
	where := nodename
	if o.t.isLocal(nodename) {
		where = "localhost"
	}
	o.t.Log().Infof("%s: replace %s by the run %s", where, head, r.Name)
	_, err = o.t.run(nodename, strings.Join(lines, "\n"))
	return err
}

func (o *execOps) deleteRuns(nodename, label, dir string, keep []string) error {
	root, all, err := o.list(nodename, label, false)
	if err != nil {
		return err
	}
	paths := make([]string, 0)
	for _, s := range btrfs.Under(all, dir) {
		rest := strings.TrimPrefix(s.Path, dir+"/")
		name, _, _ := strings.Cut(rest, "/")
		if s.Path == dir || slices.Contains(keep, name) {
			continue
		}
		paths = append(paths, s.Path)
	}
	slices.SortFunc(paths, func(a, b string) int {
		return strings.Count(b, "/") - strings.Count(a, "/")
	})
	lines := []string{"set -e"}
	if line := deleteLine(root, paths); line != "" {
		lines = append(lines, line)
	}
	// The run directories left empty, as the one of a run a receive
	// failed in before it made a subvolume.
	find := "find " + shQuote(filepath.Join(root, dir)) + " -mindepth 1 -maxdepth 1 -type d -empty"
	for _, name := range keep {
		find += " ! -name " + shQuote(name)
	}
	lines = append(lines, "if [ -d "+shQuote(filepath.Join(root, dir))+" ]; then "+find+" -delete; fi")
	if len(paths) > 0 {
		where := nodename
		if o.t.isLocal(nodename) {
			where = "localhost"
		}
		o.t.Log().Infof("%s: delete %s", where, strings.Join(paths, " "))
	}
	_, err = o.t.run(nodename, strings.Join(lines, "\n"))
	return err
}
