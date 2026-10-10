// Package mountinfo reads the mounts of the node, as /proc/self/mountinfo
// lists them: unlike /proc/mounts, it says which directory of a filesystem a
// mount shows, as the subvolume of a btrfs or the directory of a bind mount.
package mountinfo

import (
	"os"
	"strconv"
	"strings"
)

type (
	// Entry is one mount.
	Entry struct {
		// Root is the directory of the filesystem the mount shows.
		Root string

		// Target is the mount point.
		Target string

		FSType string

		// Source is the device mounted, or what stands for it, as the
		// dataset of a zfs.
		Source string

		// Options are the options of the mount, SuperOptions the ones of
		// the filesystem.
		Options      string
		SuperOptions string
	}
)

// Read returns the mounts of the node, in the order they were made.
func Read() ([]Entry, error) {
	b, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	return Parse(b), nil
}

// Parse reads the lines of a /proc/self/mountinfo:
//
//	36 35 98:0 /mnt1 /mnt2 rw,noatime master:1 - ext3 /dev/root rw,errors=continue
func Parse(b []byte) []Entry {
	l := make([]Entry, 0)
	for _, line := range strings.Split(string(b), "\n") {
		head, tail, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		hf := strings.Fields(head)
		tf := strings.Fields(tail)
		if len(hf) < 6 || len(tf) < 2 {
			continue
		}
		e := Entry{
			Root:    unescape(hf[3]),
			Target:  unescape(hf[4]),
			Options: hf[5],
			FSType:  tf[0],
			Source:  unescape(tf[1]),
		}
		if len(tf) > 2 {
			e.SuperOptions = tf[2]
		}
		l = append(l, e)
	}
	return l
}

// unescape reads the octal escapes of the fields, as \040 for a space.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
