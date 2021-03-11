package checkdf

import (
	"strings"

	"opensvc.com/opensvc/core/check"
	"opensvc.com/opensvc/util/df"
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
	Entries() ([]df.Entry, error)
	Results(*df.Entry) []*check.Result
}

// Check returns a list of check result
func Check(trans translator) ([]*check.Result, error) {
	checkResults := make([]*check.Result, 0)
	data, err := trans.Entries()
	if err != nil {
		return checkResults, err
	}
	for _, dfEntry := range data {
		if skipper(dfEntry) {
			continue
		}
		checkResults = append(checkResults, trans.Results(&dfEntry)...)
	}
	return checkResults, nil
}
