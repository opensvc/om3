//go:build linux

package asset

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// serialSources are the files the serial is read from, replaced by the
// tests.
var serialSources = struct {
	dmiDir  string
	cpuinfo string
}{
	dmiDir:  "/sys/class/dmi/id",
	cpuinfo: "/proc/cpuinfo",
}

// serialPlaceholders are the serials a firmware says when it has none, as
// v2 knew them. The kernel shows an empty file where dmidecode shows
// "Not Specified".
var serialPlaceholders = []string{"", "Unknown", "Not Specified", "0000000"}

// serial returns the serial number of the node, as v2 did: the one of the
// system, or the one of the chassis when the system has none, as a Dell
// PowerEdge 2500 or a virtual machine. A node without dmi, as a Raspberry
// Pi, has its serial in /proc/cpuinfo. A node with no serial anywhere says
// so as v2 said it: "Not Specified" with a dmi, "Unknown" without.
func serial() string {
	dmiDir := serialSources.dmiDir
	if _, err := os.Stat(dmiDir); err != nil {
		if s := cpuinfoSerial(); s != "" {
			return s
		}
		return "Unknown"
	}
	for _, name := range []string{"product_serial", "chassis_serial"} {
		if s := readTrimmed(filepath.Join(dmiDir, name)); !slices.Contains(serialPlaceholders, s) {
			return s
		}
	}
	return "Not Specified"
}

// cpuinfoSerial returns the Serial line of /proc/cpuinfo, empty when there
// is none.
func cpuinfoSerial() string {
	f, err := os.Open(serialSources.cpuinfo)
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		k, v, ok := strings.Cut(s.Text(), ":")
		if ok && strings.TrimSpace(k) == "Serial" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func readTrimmed(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
