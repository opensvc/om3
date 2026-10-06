// Package chkzpool is the zpool check driver: the health of each pool, 0
// when online, a positive value for each degraded state.
package chkzpool

import (
	"context"
	"errors"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
	"github.com/opensvc/om3/v3/core/resource"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "zpool"
	// DriverName is the name of check driver.
	DriverName = "zpool"
)

type (
	checker struct{}

	pool struct {
		name   string
		health int64
	}

	poolNamer interface {
		PoolName() string
	}
)

// healths are the values of the pool states, as v2 reported them.
var healths = map[string]int64{
	"ONLINE":   0,
	"DEGRADED": 1,
	"FAULTED":  2,
	"OFFLINE":  3,
	"REMOVED":  4,
	"UNAVAIL":  5,
}

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	b, err := checkexec.Output(ctx, "zpool", "list", "-H", "-o", "name,health")
	if errors.Is(err, checkexec.ErrNotFound) {
		return rs, nil
	} else if err != nil {
		return rs, err
	}
	for _, p := range parse(b) {
		rs.Push(check.Result{
			DriverGroup: DriverGroup,
			DriverName:  DriverName,
			Instance:    p.name,
			Value:       p.health,
			Path: check.ObjectPathWithResource(ctx, objs, func(r resource.Driver) bool {
				i, ok := r.(poolNamer)
				return ok && i.PoolName() == p.name
			}),
		})
	}
	return rs, nil
}

// parse reads the name and health of the pools in a zpool list -H output. A
// state not known is reported 6.
func parse(b []byte) []pool {
	var l []pool
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		health, ok := healths[fields[1]]
		if !ok {
			health = 6
		}
		l = append(l, pool{name: fields[0], health: health})
	}
	return l
}
