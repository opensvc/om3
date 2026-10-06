// Package chkfszfs is the zfs fs_u check driver: the usage of each zfs
// dataset, as the df driver reports the usage of each mounted filesystem.
package chkfszfs

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "fs_u"
	// DriverName is the name of check driver.
	DriverName = "zfs"
)

type (
	checker struct{}

	dataset struct {
		name       string
		used       int64
		avail      int64
		mountPoint string
	}
)

// containerLayer matches the datasets a container engine makes for its
// layers, which are not the node's to watch.
var containerLayer = regexp.MustCompile(`/[0-9a-f]{64}`)

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	b, err := checkexec.OutputIn(ctx, "", []string{"/usr/sbin", "/sbin"}, "zfs", "list", "-H", "-p", "-o", "name,used,avail,mountpoint")
	if errors.Is(err, checkexec.ErrNotFound) {
		return rs, nil
	} else if err != nil {
		return rs, err
	}
	for _, ds := range parse(b) {
		path := ""
		if strings.HasPrefix(ds.mountPoint, "/") {
			path = check.ObjectPathClaimingDir(ctx, ds.mountPoint, objs)
		}
		total := ds.used + ds.avail
		var pct int64
		if total > 0 {
			pct = (100*ds.used + total/2) / total
		}
		for _, r := range []check.Result{
			{Instance: ds.name, Value: pct, Unit: "%"},
			{Instance: ds.name + ".free", Value: ds.avail / 1024, Unit: "kb"},
			{Instance: ds.name + ".size", Value: total / 1024, Unit: "kb"},
		} {
			r.DriverGroup = DriverGroup
			r.DriverName = DriverName
			r.Path = path
			rs.Push(r)
		}
	}
	return rs, nil
}

// parse reads the datasets of a zfs list -H -p output, leaving out the
// snapshots, the container layers and the datasets of a sync in progress.
func parse(b []byte) []dataset {
	var l []dataset
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			continue
		}
		name := fields[0]
		if strings.Contains(name, "@") || containerLayer.MatchString(name) || strings.Contains(name, "osvc_sync_") {
			continue
		}
		used, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		avail, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			continue
		}
		l = append(l, dataset{name: name, used: used, avail: avail, mountPoint: fields[3]})
	}
	return l
}
