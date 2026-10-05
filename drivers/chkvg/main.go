// Package chkvg is the vg_u check driver: the percentage of each LVM volume
// group allocated.
package chkvg

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
	"github.com/opensvc/om3/v3/core/resource"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "vg_u"
	// DriverName is the name of check driver.
	DriverName = "lvm"
)

type (
	checker struct{}

	vg struct {
		name    string
		usedPct int64
	}

	vgNamer interface {
		VolumeGroupName() string
	}
)

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	b, err := checkexec.OutputIn(ctx, "", []string{"/sbin", "/usr/sbin"}, "vgs", "--units", "b", "--noheadings", "-o", "vg_name,vg_size,vg_free")
	if errors.Is(err, checkexec.ErrNotFound) {
		return rs, nil
	} else if err != nil {
		return rs, err
	}
	for _, v := range parse(b) {
		rs.Push(check.Result{
			DriverGroup: DriverGroup,
			DriverName:  DriverName,
			Instance:    v.name,
			Value:       v.usedPct,
			Unit:        "%",
			Path: check.ObjectPathWithResource(ctx, objs, func(r resource.Driver) bool {
				i, ok := r.(vgNamer)
				return ok && i.VolumeGroupName() == v.name
			}),
		})
	}
	return rs, nil
}

// parse reads the volume groups of a vgs --units b output, sizes ending with
// the B unit.
func parse(b []byte) []vg {
	var l []vg
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		size, err := strconv.ParseInt(strings.TrimSuffix(fields[1], "B"), 10, 64)
		if err != nil || size == 0 {
			continue
		}
		free, err := strconv.ParseInt(strings.TrimSuffix(fields[2], "B"), 10, 64)
		if err != nil {
			continue
		}
		l = append(l, vg{name: fields[0], usedPct: 100 * (size - free) / size})
	}
	return l
}
