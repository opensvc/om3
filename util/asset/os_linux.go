//go:build linux

package asset

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"
)

type (
	// osProbe tells the vendor and the release of the distribution of the
	// system its root directory holds, the way OpenSVC v2 tells them, as
	// the filtersets of the collector and the configurations they deploy
	// match the values v2 reported.
	osProbe struct {
		root      string
		osRelease map[string]string
	}
)

// osVendors are the vendors of the distributions, by their os-release id.
var osVendors = map[string]string{
	"alpine":   "Alpine",
	"debian":   "Debian",
	"ubuntu":   "Ubuntu",
	"arch":     "Arch",
	"vmware":   "VMware",
	"oracle":   "Oracle",
	"sles":     "SuSE",
	"opensuse": "SuSE",
	"rhel":     "Red Hat",
	"centos":   "CentOS",
	"fedora":   "Fedora",
	"caasp":    "SuSE",
	"gentoo":   "Gentoo",
}

func newOSProbe(root string) *osProbe {
	t := &osProbe{root: root}
	t.osRelease = t.parseOSRelease()
	return t
}

func (t *osProbe) path(p string) string {
	return filepath.Join(t.root, p)
}

func (t *osProbe) exists(p string) bool {
	_, err := os.Stat(t.path(p))
	return err == nil
}

func (t *osProbe) read(p string) (string, bool) {
	b, err := os.ReadFile(t.path(p))
	if err != nil {
		return "", false
	}
	return string(b), true
}

// lines are the lines of the file, none for a file it can not read.
func (t *osProbe) lines(p string) []string {
	s, ok := t.read(p)
	if !ok {
		return nil
	}
	l := make([]string, 0)
	scanner := bufio.NewScanner(bytes.NewBufferString(s))
	for scanner.Scan() {
		l = append(l, scanner.Text())
	}
	return l
}

// lastFieldValue is the value of a "key=value" line, its text after the
// last "=", stripped of the cutset.
func lastFieldValue(line, cutset string) string {
	l := strings.Split(line, "=")
	return strings.Trim(l[len(l)-1], cutset)
}

// parseOSRelease returns the variables of /etc/os-release, by their name
// lowercased, their value unquoted.
func (t *osProbe) parseOSRelease() map[string]string {
	m := make(map[string]string)
	for _, line := range t.lines("/etc/os-release") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[strings.ToLower(k)] = strings.Trim(v, `"`)
	}
	return m
}

// Vendor returns the vendor of the distribution, as "Ubuntu" or "Red Hat".
func (t *osProbe) Vendor() string {
	if v, ok := osVendors[t.osRelease["id"]]; ok {
		return v
	}
	for _, line := range t.lines("/etc/lsb-release") {
		if strings.Contains(line, "DISTRIB_ID") {
			return lastFieldValue(line, `"`)
		}
	}
	switch {
	case t.exists("/etc/debian_version"):
		return "Debian"
	case t.exists("/etc/SuSE-release"):
		return "SuSE"
	case t.exists("/etc/vmware-release"):
		return "VMware"
	case t.exists("/etc/oracle-release"):
		return "Oracle"
	}
	if s, ok := t.read("/etc/redhat-release"); ok {
		switch {
		case strings.Contains(s, "CentOS"):
			return "CentOS"
		case strings.Contains(s, "Oracle"):
			return "Oracle"
		default:
			return "Red Hat"
		}
	}
	if t.exists("/etc/alpine-release") {
		return "Alpine"
	}
	if s := t.osRelease["name"]; s != "" {
		return s
	}
	return "Unknown"
}

// withoutVendor returns s with the vendor of the distribution removed,
// whatever its case.
func (t *osProbe) withoutVendor(s string) string {
	v := t.Vendor()
	if v == "" {
		return s
	}
	re, err := regexp.Compile("(?i)" + v)
	if err != nil {
		re = regexp.MustCompile("(?i)" + regexp.QuoteMeta(v))
	}
	return re.ReplaceAllString(s, "")
}

func (t *osProbe) releaseFromOSRelease() string {
	s := t.osRelease["pretty_name"]
	if s == "" {
		return ""
	}
	return strings.TrimSpace(t.withoutVendor(s))
}

func (t *osProbe) releaseFromLSB() string {
	var s string
	for _, line := range t.lines("/etc/lsb-release") {
		if strings.Contains(line, "DISTRIB_RELEASE") {
			if s = lastFieldValue(line, `"`); s != "" {
				break
			}
		}
		if strings.Contains(line, "DISTRIB_DESCRIPTION") {
			if s = lastFieldValue(line, `"`); s != "" {
				break
			}
		}
	}
	if s == "" {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(s, t.Vendor(), ""))
}

// Release returns the release of the distribution, as "24.04.5 LTS" or
// "Enterprise Linux 7.9".
func (t *osProbe) Release() string {
	s := t.releaseFromOSRelease()
	if s == "Enterprise Linux" && t.osRelease["version"] != "" {
		// Before el8, the pretty name did not have the version.
		return s + " " + t.osRelease["version"]
	}
	switch s {
	case "", "/Linux", "Linux 7 (Core)":
		// No pretty name, or one poor in information, as the centos7 one.
	default:
		return s
	}
	if lines := t.lines("/etc/SuSE-release"); lines != nil {
		l := make([]string, 0)
		for _, line := range lines {
			if strings.Contains(line, "VERSION") {
				l = append(l, lastFieldValue(line, `" `))
			}
			if strings.Contains(line, "PATCHLEVEL") {
				l = append(l, lastFieldValue(line, `" `))
			}
		}
		return strings.Join(l, ".")
	}
	if s, ok := t.read("/etc/alpine-release"); ok {
		return strings.TrimSpace(s)
	}
	if s := t.releaseFromLSB(); s != "" {
		return s
	}
	if s, ok := t.read("/etc/debian_version"); ok {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	if oracle, ok := t.read("/etc/oracle-release"); ok && strings.Contains(oracle, " VM ") {
		if redhat, ok := t.read("/etc/redhat-release"); ok {
			first, _, _ := strings.Cut(redhat, "\n")
			return strings.TrimSpace(strings.ReplaceAll(first, t.Vendor(), ""))
		}
	}
	for _, p := range []string{
		"/etc/debian_version",
		"/etc/vmware-release",
		"/etc/oracle-release",
		"/etc/redhat-release",
		"/etc/gentoo-release",
	} {
		if !t.exists(p) {
			continue
		}
		s, ok := t.read(p)
		if !ok {
			return "Unknown"
		}
		s, _, _ = strings.Cut(s, "\n")
		for _, word := range []string{t.Vendor(), "GNU/Linux", "Linux", "release"} {
			s = strings.ReplaceAll(s, word, "")
		}
		return strings.TrimSpace(s)
	}
	return "Unknown"
}

// uname returns the system name, the kernel release and the machine of the
// system, as "Linux", "6.8.0-142-generic" and "x86_64".
func uname() (sysname, release, machine string, err error) {
	var u unix.Utsname
	if err = unix.Uname(&u); err != nil {
		return
	}
	sysname = unix.ByteSliceToString(u.Sysname[:])
	release = unix.ByteSliceToString(u.Release[:])
	machine = unix.ByteSliceToString(u.Machine[:])
	return
}
