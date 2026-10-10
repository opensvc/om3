package fssnap

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/opensvc/om3/v3/util/plog"
)

// killHolders kills the processes holding something below p: a file open, a
// mapping, or their working or root directory. It says how many it killed,
// once they are gone or a few seconds passed.
//
// fuser -m is not enough: it finds the processes using the filesystem of the
// mount point, and a btrfs snapshot read below the mount of the root of its
// filesystem is a subvolume, a filesystem of its own to fuser.
func killHolders(log *plog.Logger, p string) int {
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	killed := make([]int, 0)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		if !holds(pid, p) {
			continue
		}
		cmdline, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		log.Warnf("kill %d (%s), which holds %s", pid, strings.TrimSpace(strings.ReplaceAll(string(cmdline), "\x00", " ")), p)
		if err := syscall.Kill(pid, syscall.SIGKILL); err == nil {
			killed = append(killed, pid)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for _, pid := range killed {
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err != nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	return len(killed)
}

// holds says the process pid holds something below p.
func holds(pid int, p string) bool {
	dir := filepath.Join("/proc", strconv.Itoa(pid))
	for _, name := range []string{"cwd", "root", "exe"} {
		if target, err := os.Readlink(filepath.Join(dir, name)); err == nil && isUnder(cleanLink(target), p) {
			return true
		}
	}
	if fds, err := os.ReadDir(filepath.Join(dir, "fd")); err == nil {
		for _, fd := range fds {
			if target, err := os.Readlink(filepath.Join(dir, "fd", fd.Name())); err == nil && isUnder(cleanLink(target), p) {
				return true
			}
		}
	}
	f, err := os.Open(filepath.Join(dir, "maps"))
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 6 && isUnder(cleanLink(strings.Join(fields[5:], " ")), p) {
			return true
		}
	}
	return false
}

// cleanLink drops the " (deleted)" the kernel appends to the path of a file
// removed while open.
func cleanLink(s string) string {
	return strings.TrimSuffix(s, " (deleted)")
}
