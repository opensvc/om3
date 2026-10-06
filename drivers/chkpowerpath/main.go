// Package chkpowerpath is the PowerPath check driver: the number of active
// and alive paths of each PowerPath pseudo device.
package chkpowerpath

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "mpath"
	// DriverName is the name of check driver.
	DriverName = "powerpath"
)

type (
	checker struct{}

	pseudo struct {
		name        string
		paths       []string
		activePaths int64
	}
)

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	b, err := checkexec.OutputIn(ctx, "", []string{"/sbin", "/usr/sbin"}, "powermt", "display", "dev=all")
	if errors.Is(err, checkexec.ErrNotFound) {
		return rs, nil
	} else if err != nil {
		return rs, err
	}
	devices := check.DeviceIndexOf(ctx, objs)
	for _, p := range parse(b) {
		rs.Push(check.Result{
			DriverGroup: DriverGroup,
			DriverName:  DriverName,
			Instance:    instance(p),
			Value:       p.activePaths,
			Path:        devices.Path(append([]string{"/dev/" + p.name}, p.paths...)...),
		})
	}
	return rs, nil
}

// instance names the pseudo device by the wwid of its first path, as the
// native multipath driver names a map, or else by its name.
func instance(p pseudo) string {
	if len(p.paths) > 0 {
		b, err := os.ReadFile(filepath.Join("/sys/block", filepath.Base(p.paths[0]), "device", "wwid"))
		if wwid := strings.TrimPrefix(strings.TrimSpace(string(b)), "naa."); err == nil && wwid != "" {
			return wwid
		}
	}
	return p.name
}

// parse reads the pseudo devices of a powermt display dev=all output, blocks
// separated by an empty line, each starting with its pseudo name.
func parse(b []byte) []pseudo {
	var (
		l   []pseudo
		cur *pseudo
	)
	flush := func() {
		if cur != nil {
			l = append(l, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if _, name, ok := strings.Cut(line, "Pseudo name="); ok {
			flush()
			cur = &pseudo{name: strings.TrimSpace(name)}
			continue
		}
		if cur == nil {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if strings.HasPrefix(fields[2], "sd") {
			cur.paths = append(cur.paths, "/dev/"+fields[2])
		}
		if strings.Contains(line, "active") && strings.Contains(line, "alive") {
			cur.activePaths++
		}
	}
	flush()
	return l
}
