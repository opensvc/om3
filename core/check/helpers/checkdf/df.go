package checkdf

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/util/df"
)

func skipper(dfEntry df.Entry) bool {
	// discard bind mounts: we get metric from the source anyway
	var searched string
	device := dfEntry.Device
	if strings.HasPrefix(device, "/") && !strings.HasPrefix(device, "/dev") && !strings.HasPrefix(device, "//") {
		return true
	}

	switch device {
	case "overlay":
		return true
	case "overlay2":
		return true
	case "aufs":
		return true
	}

	for _, searched = range []string{"osvc_sync_"} {
		if strings.Contains(device, searched) {
			return true
		}
	}

	mountPoint := dfEntry.MountPoint
	for _, prefix := range []string{"/Volumes", "/media/", "/run", "/sys/", "/shm", "/snap/"} {
		if strings.HasPrefix(mountPoint, prefix) {
			return true
		}
	}

	for _, searched = range []string{"/overlay2/", "/snapd/", "/graph/", "/aufs/mnt/"} {
		if strings.Contains(mountPoint, searched) {
			return true
		}
	}

	return false
}

type translator interface {
	Entries(context.Context) ([]df.Entry, error)
	ResultSet(context.Context, *df.Entry, []interface{}) *check.ResultSet
}

// Check returns a list of check result
func Check(ctx context.Context, trans translator, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	data, err := trans.Entries(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return rs, err
	}
	for _, dfEntry := range data {
		if skipper(dfEntry) {
			continue
		}
		rs.Add(trans.ResultSet(ctx, &dfEntry, objs))
	}
	return rs, nil
}
