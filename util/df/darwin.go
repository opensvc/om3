// +build darwin

package df

import (
	"os/exec"
	"strconv"
	"strings"
)

func doDFInode() ([]byte, error) {
	return doDF([]string{"-i"})
}

func doDFUsage() ([]byte, error) {
	return doDF([]string{"-k", "-P"})
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

// Usage executes and parses a df command
func Usage() ([]Entry, error) {
	b, err := doDFUsage()
	if err != nil {
		return nil, err
	}
	return parseUsage(b)
}

// Inode executes and parses a df command
func Inode() ([]Entry, error) {
	b, err := doDFInode()
	if err != nil {
		return nil, err
	}
	return parseInodes(b)
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

func parseInodes(b []byte) ([]Entry, error) {
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
