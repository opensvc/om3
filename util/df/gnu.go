// +build linux solaris

package df

import (
	"os/exec"
	"strconv"
	"strings"
)

// TypeMountUsage executes and parses a df command for a mount point and a fstype
func TypeMountUsage(fstype string, mnt string) ([]Entry, error) {
	b, err := doDF([]string{"-lP", "-t", fstype, mnt})
	if err != nil {
		return nil, err
	}
	return parse(b)
}

// MountUsage executes and parses a df command for a mount point
func MountUsage(mnt string) ([]Entry, error) {
	b, err := doDF([]string{"-lP", mnt})
	if err != nil {
		return nil, err
	}
	return parse(b)
}

func doDFInode() ([]byte, error) {
	return doDF([]string{"-lPi"})
}

func doDFUsage() ([]byte, error) {
	return doDF([]string{"-lP"})
}

func doDF(args []string) ([]byte, error) {
	df, err := exec.LookPath("df")
	if err != nil {
		return nil, err
	}
	cmd := &exec.Cmd{
		Path: df,
		Args: args,
	}
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return b, nil
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
			Total:       total,
			Used:        used,
			Free:        free,
			UsedPercent: 100 * used / total,
			MountPoint:  l[5],
		})
	}
	return r, nil
}
