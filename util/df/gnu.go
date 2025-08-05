//go:build linux || solaris

package df

import (
	"strconv"
	"strings"
)

const (
	typeOption = "-t"
)

func doDFInode(args ...string) ([]byte, error) {
	return doDF(append([]string{"-lPi"}, args...))
}

func doDFUsage(args ...string) ([]byte, error) {
	return doDF(append([]string{"-lP"}, args...))
}

func parse(b []byte) ([]Entry, error) {
	r := make([]Entry, 0)
	text := string(b)
	for _, line := range strings.Split(text, "\n")[1:] {
		l := strings.Fields(line)
		if len(l) != 6 {
			continue
		}
		total, err := strconv.ParseInt(l[1], 10, 64)
		if err != nil {
			continue
		}
		used, err := strconv.ParseInt(l[2], 10, 64)
		if err != nil {
			continue
		}
		free, err := strconv.ParseInt(l[3], 10, 64)
		if err != nil {
			continue
		}
		r = append(r, Entry{
			Device:      l[0],
			Total:       total * 1024,
			Used:        used * 1024,
			Free:        free * 1024,
			UsedPercent: 100 * used / total,
			MountPoint:  l[5],
		})
	}
	return r, nil
}

func parseUsage(b []byte) ([]Entry, error) {
	return parse(b)
}

func parseInode(b []byte) ([]Entry, error) {
	return parse(b)
}
