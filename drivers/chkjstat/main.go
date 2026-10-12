// Package chkjstat is the jstat check driver: the statistics jstat reports
// of the JVMs a resource of an object started, each attributed to that
// object, by the environment the agent gave the process.
package chkjstat

import (
	"bytes"
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "jstat"
	// DriverName is the name of check driver.
	DriverName = "jstat"
)

type (
	checker struct{}

	// jvm is a java process an object started.
	jvm struct {
		pid      string
		instance string
		path     string

		// uid and gid are the owner of the process, jstat runs as.
		uid uint32
		gid uint32
	}
)

// stats are the jstat options reported. An option the JVM does not know, as
// gcpermcapacity on a JVM without a permanent generation, is skipped.
var stats = []string{
	"class",
	"gc",
	"gccapacity",
	"gcnew",
	"gcnewcapacity",
	"gcold",
	"gcoldcapacity",
	"gcpermcapacity",
	"gcutil",
}

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	for _, j := range jvms() {
		java, err := os.Readlink(filepath.Join("/proc", j.pid, "exe"))
		if err != nil {
			continue
		}
		jstat := filepath.Join(filepath.Dir(java), "jstat")
		if !isTrustedBinary(jstat, j.uid) {
			continue
		}
		for _, stat := range stats {
			ctx, cancel := context.WithTimeout(ctx, checkexec.Timeout)
			cmd := exec.CommandContext(ctx, jstat, "-"+stat, j.pid)
			// jstat reads the statistics the JVM shares with its own
			// user, and is a binary of the JVM installation, which that
			// user may own: it runs as that user, not as root.
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: j.uid, Gid: j.gid}}
			b, err := cmd.Output()
			cancel()
			if err != nil {
				continue
			}
			for _, m := range parse(b) {
				rs.Push(check.Result{
					DriverGroup: DriverGroup,
					DriverName:  DriverName,
					Instance:    j.instance + "." + stat + "." + m.name,
					Value:       m.value,
					Path:        j.path,
				})
			}
		}
	}
	return rs, nil
}

// jvms returns the java processes whose environment names the resource or
// the check instance they are reported as.
//
// Any process can name itself java and set that environment. One of another
// mount namespace, as a container, is skipped: the path of its executable is
// a path of its own filesystem, which names another file here.
func jvms() []jvm {
	dirs, _ := filepath.Glob("/proc/[0-9]*")
	selfNS, err := os.Readlink("/proc/self/ns/mnt")
	if err != nil {
		return nil
	}
	var l []jvm
	for _, dir := range dirs {
		comm, err := os.ReadFile(filepath.Join(dir, "comm"))
		if err != nil || strings.TrimSpace(string(comm)) != "java" {
			continue
		}
		if ns, err := os.Readlink(filepath.Join(dir, "ns", "mnt")); err != nil || ns != selfNS {
			continue
		}
		fi, err := os.Stat(dir)
		if err != nil {
			continue
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		environ, err := os.ReadFile(filepath.Join(dir, "environ"))
		if err != nil {
			continue
		}
		if j, ok := fromEnviron(filepath.Base(dir), environ); ok {
			j.uid, j.gid = st.Uid, st.Gid
			l = append(l, j)
		}
	}
	return l
}

// isTrustedBinary reports whether the binary at p may be run for a process
// of uid: a regular file owned by root or by uid, that no one else may
// write.
func isTrustedBinary(p string, uid uint32) bool {
	fi, err := os.Stat(p)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o022 != 0 {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return st.Uid == 0 || st.Uid == uid
}

// fromEnviron returns the jvm a process environment describes: its instance
// is OPENSVC_CHK_INSTANCE, or else the rid of the resource that started it,
// and its object OPENSVC_SVCPATH, or else OPENSVC_SVCNAME.
func fromEnviron(pid string, environ []byte) (jvm, bool) {
	env := make(map[string]string)
	for _, kv := range bytes.Split(environ, []byte{0}) {
		if k, v, ok := strings.Cut(string(kv), "="); ok {
			env[k] = v
		}
	}
	j := jvm{pid: pid, instance: env["OPENSVC_CHK_INSTANCE"], path: env["OPENSVC_SVCPATH"]}
	if j.instance == "" {
		j.instance = env["OPENSVC_RID"]
	}
	if j.instance == "" {
		return j, false
	}
	if j.path == "" {
		j.path = env["OPENSVC_SVCNAME"]
	}
	return j, true
}

type metric struct {
	name  string
	value int64
}

// parse reads the header and value lines of a jstat output, the values
// rounded, a decimal comma read as a point.
func parse(b []byte) []metric {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 {
		return nil
	}
	headers := strings.Fields(lines[0])
	values := strings.Fields(strings.ReplaceAll(lines[1], ",", "."))
	var l []metric
	for i, h := range headers {
		if i >= len(values) {
			break
		}
		v, err := strconv.ParseFloat(values[i], 64)
		if err != nil {
			continue
		}
		l = append(l, metric{name: h, value: int64(math.Round(v))})
	}
	return l
}
