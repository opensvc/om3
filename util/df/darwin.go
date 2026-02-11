//go:build darwin

package df

import (
	"context"
	"strconv"
	"strings"
)

const (
	typeOption = "-T"
)

func doDFInode(ctx context.Context, args ...string) ([]byte, error) {
	return doDF(ctx, append([]string{"-i"}, args...))
}

func doDFUsage(ctx context.Context, args ...string) ([]byte, error) {
	return doDF(ctx, append([]string{"-k", "-P"}, args...))
}

func parseUsage(b []byte) ([]Entry, error) {
	r := make([]Entry, 0)
	text := string(b)
	for _, line := range strings.Split(text, "\n")[1:] {
		l := strings.Fields(line)
		if len(l) != (6) {
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
			Total:       total,
			Used:        used,
			Free:        free,
			UsedPercent: 100 * used / total,
			MountPoint:  l[5],
		})
	}
	return r, nil
}

func parseInode(b []byte) ([]Entry, error) {
	r := make([]Entry, 0)
	text := string(b)
	for _, line := range strings.Split(text, "\n")[1:] {
		l := strings.Fields(line)
		if len(l) != (9) {
			continue
		}
		used, err := strconv.ParseInt(l[5], 10, 64)
		if err != nil {
			continue
		}
		free, err := strconv.ParseInt(l[6], 10, 64)
		if err != nil {
			continue
		}
		total := used + free
		r = append(r, Entry{
			Device:      l[0],
			Total:       total,
			Used:        used,
			Free:        free,
			UsedPercent: 100 * used / total,
			MountPoint:  l[8],
		})
	}
	return r, nil
}
