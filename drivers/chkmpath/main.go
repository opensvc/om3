// Package chkmpath is the native multipath check driver: the number of
// active paths of each multipath device, as multipathd reports them.
package chkmpath

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "mpath"
	// DriverName is the name of check driver.
	DriverName = "multipathd"
)

type (
	checker struct{}

	mpath struct {
		wwid string

		// devs are the devices of the map, to attribute it to the object
		// using it: the dm device and the last active path.
		devs []string

		activePaths int64
	}
)

// hctl matches the host:channel:target:lun address a path line names its
// device after, whatever the tree drawn before it.
var hctl = regexp.MustCompile(`^[0-9]+:[0-9]+:[0-9]+:[0-9]+$`)

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	b, err := checkexec.OutputIn(ctx, "", []string{"/sbin", "/usr/sbin"}, "multipathd", "-kshow topo")
	if errors.Is(err, checkexec.ErrNotFound) {
		return rs, nil
	} else if err != nil {
		return rs, err
	}
	devices := check.DeviceIndexOf(ctx, objs)
	for _, m := range parse(b) {
		rs.Push(check.Result{
			DriverGroup: DriverGroup,
			DriverName:  DriverName,
			Instance:    m.wwid,
			Value:       m.activePaths,
			Path:        devices.Path(m.devs...),
		})
	}
	return rs, nil
}

// parse reads the maps of a multipathd show topo output: a line naming a dm
// device starts a map, and its active and ready path lines are counted.
func parse(b []byte) []mpath {
	var (
		l   []mpath
		cur *mpath
	)
	flush := func() {
		if cur != nil {
			l = append(l, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, " dm-") {
			flush()
			fields := strings.Fields(strings.TrimPrefix(line, ": "))
			if len(fields) > 0 && strings.HasSuffix(fields[0], ":") {
				// An event prefix: create, switchpg, reload...
				fields = fields[1:]
			}
			if len(fields) < 2 {
				continue
			}
			m := mpath{}
			if strings.HasPrefix(fields[1], "(") {
				// <alias> (<wwid>) dm-<n> ...
				m.wwid = strings.Trim(fields[1], "()")
			} else {
				m.wwid = fields[0]
			}
			if n := len(m.wwid); (n == 17 || n == 33) && strings.ContainsRune("235", rune(m.wwid[0])) {
				// The NAA type prefix of the wwid, as v2 left it out.
				m.wwid = m.wwid[1:]
			}
			for _, f := range fields {
				if strings.HasPrefix(f, "dm-") {
					m.devs = append(m.devs, "/dev/"+f)
				}
			}
			cur = &m
			continue
		}
		if cur == nil {
			continue
		}
		if strings.Contains(line, "active ready") || strings.Contains(line, "[active][ready]") {
			cur.activePaths++
			if dev := pathDevice(line); dev != "" {
				cur.devs = append(cur.devs[:min(len(cur.devs), 1)], "/dev/"+dev)
			}
		}
	}
	flush()
	return l
}

// pathDevice returns the device a path line names, the field after its
// address.
func pathDevice(line string) string {
	fields := strings.Fields(line)
	for i, f := range fields[:max(len(fields)-1, 0)] {
		if hctl.MatchString(f) {
			return fields[i+1]
		}
	}
	return ""
}
