package rescontainerocibase

import (
	"fmt"
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
