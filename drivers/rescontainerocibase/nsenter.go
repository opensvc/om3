package rescontainerocibase

import (
	"fmt"
	iofs "io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	// nsenterHelp returns the help of nsenter, which tells the options it
	// knows, a variable for the tests.
	nsenterHelp = func() string {
		b, _ := exec.Command("nsenter", "--help").CombinedOutput()
		return string(b)
	}

	// procRoot is the root of the process information, a variable for the
	// tests.
	procRoot = "/proc"

	nsenterOptionsOnce sync.Once
	nsenterHasAll      bool
	nsenterHasCgroup   bool
)

// nsenterArgs returns the command entering every namespace of the process
// pid, as the process of a container.
//
// It is "nsenter -t <pid> --all" where nsenter knows --all. An older one, as
// util-linux 2.23 of rhel7, does not: it gets the namespaces named instead,
// mount, uts, ipc, net and pid, and the user and cgroup ones only where the
// process has its own, as --all does, entering one's own user namespace
// failing.
func nsenterArgs(pid int) []string {
	nsenterOptionsOnce.Do(func() {
		help := nsenterHelp()
		nsenterHasAll = strings.Contains(help, "--all")
		nsenterHasCgroup = strings.Contains(help, "--cgroup")
	})
	args := []string{"nsenter", "-t", fmt.Sprint(pid)}
	if nsenterHasAll {
		return append(args, "--all")
	}
	args = append(args, "-m", "-u", "-i", "-n", "-p")
	if hasOwnNamespace(pid, "user") {
		args = append(args, "-U")
	}
	if nsenterHasCgroup && hasOwnNamespace(pid, "cgroup") {
		args = append(args, "-C")
	}
	return args
}

// hasOwnNamespace says the process pid has a namespace of kind ns of its
// own, another than the one of the agent.
func hasOwnNamespace(pid int, ns string) bool {
	theirs, err := os.Readlink(filepath.Join(procRoot, fmt.Sprint(pid), "ns", ns))
	if err != nil {
		return false
	}
	ours, err := os.Readlink(filepath.Join(procRoot, "self", "ns", ns))
	if err != nil {
		return false
	}
	return theirs != ours
}

// maxLinkHops bounds the links followed resolving a path in a container, as
// the kernel bounds them, a loop of links resolving to nothing.
const maxLinkHops = 40

// containerHasExecutable says the file of path, absolute in the container of
// the process pid, is an executable regular file, its links resolved as the
// container sees them: an absolute link, as the /bin/sh -> /bin/busybox of
// the images of busybox or alpine, leads into the files of the container, not
// of the node.
func containerHasExecutable(pid int, path string) bool {
	root, err := os.OpenRoot(filepath.Join(procRoot, fmt.Sprint(pid), "root"))
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()
	fi, ok := resolveInRoot(root, path)
	return ok && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0
}

// resolveInRoot returns the file info of path in root, following its links
// component by component, an absolute link target restarting at root, and
// false when it does not exist or the links loop.
func resolveInRoot(root *os.Root, path string) (iofs.FileInfo, bool) {
	todo := components(path)
	var done []string
	for hops := 0; len(todo) > 0; {
		name := todo[0]
		todo = todo[1:]
		if name == ".." {
			if len(done) > 0 {
				done = done[:len(done)-1]
			}
			continue
		}
		cur := filepath.Join(append(append([]string{}, done...), name)...)
		fi, err := root.Lstat(cur)
		if err != nil {
			return nil, false
		}
		if fi.Mode()&iofs.ModeSymlink == 0 {
			if len(todo) == 0 {
				return fi, true
			}
			done = append(done, name)
			continue
		}
		hops++
		if hops > maxLinkHops {
			return nil, false
		}
		target, err := root.Readlink(cur)
		if err != nil {
			return nil, false
		}
		if filepath.IsAbs(target) {
			done = nil
		}
		todo = append(components(target), todo...)
	}
	// The root itself: a directory.
	fi, err := root.Stat(".")
	return fi, err == nil
}

// components returns the names of path, without the empty ones and ".".
func components(path string) []string {
	var l []string
	for _, name := range strings.Split(filepath.ToSlash(path), "/") {
		if name != "" && name != "." {
			l = append(l, name)
		}
	}
	return l
}
