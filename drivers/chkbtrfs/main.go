// Package chkbtrfs is the btrfs check driver: the error counters of the
// devices of each mounted btrfs filesystem.
package chkbtrfs

import (
	"bufio"
	"context"
	"errors"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "btrfs"
	// DriverName is the name of check driver.
	DriverName = "btrfs"
)

type (
	checker struct{}

	// stat is an error counter of a device.
	stat struct {
		dev   string
		name  string
		value int64
	}
)

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	if checkexec.Find("btrfs", "/sbin", "/usr/sbin") == "" {
		return rs, nil
	}
	// A device of several mounts, as the subvolumes of a filesystem, is
	// reported once.
	stats := make(map[string]stat)
	for _, mnt := range mountPoints() {
		b, err := checkexec.OutputIn(ctx, "", []string{"/sbin", "/usr/sbin"}, "btrfs", "device", "stats", mnt)
		if errors.Is(err, checkexec.ErrNotFound) {
			return rs, nil
		} else if err != nil {
			continue
		}
		for _, s := range parse(b) {
			stats[s.dev+"."+s.name] = s
		}
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var devices check.DeviceIndex
	if len(keys) > 0 {
		devices = check.DeviceIndexOf(ctx, objs)
	}
	for _, k := range keys {
		s := stats[k]
		rs.Push(check.Result{
			DriverGroup: DriverGroup,
			DriverName:  DriverName,
			Instance:    k,
			Value:       s.value,
			Path:        devices.Path(s.dev),
		})
	}
	return rs, nil
}

// mountPoints returns the mount points of the btrfs filesystems.
func mountPoints() []string {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var l []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || fields[2] != "btrfs" {
			continue
		}
		if _, err := os.Stat(fields[1]); err == nil {
			l = append(l, fields[1])
		}
	}
	return l
}

// parse reads the counters of a btrfs device stats output, its lines as
// "[/dev/sdb].write_io_errs   0".
func parse(b []byte) []stat {
	var l []stat
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		dev, name, ok := strings.Cut(fields[0], "].")
		if !ok || !strings.HasPrefix(dev, "[") {
			continue
		}
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		l = append(l, stat{dev: strings.TrimPrefix(dev, "["), name: name, value: v})
	}
	return l
}
