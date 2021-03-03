// +build linux

package df

import (
	"os/exec"
	"strconv"
	"strings"
)

func doDF() ([]byte, error) {
	df, err := exec.LookPath("df")
	if err != nil {
		return nil, err
	}
	cmd := &exec.Cmd{
		Path: df,
		Args: []string{"-lP"},
	}
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Do executes and parses a df command
func Do() ([]DFEntry, error) {
	r := make([]DFEntry, 0)
	b, err := doDF()
	if err != nil {
		return r, err
	}
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
		r = append(r, DFEntry{
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
