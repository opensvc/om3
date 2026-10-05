// Package chkomreport is the Dell OpenManage check driver: the state of the
// system and chassis components omreport lists, 0 when ok.
package chkomreport

import (
	"context"
	"errors"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
)

const (
	// DriverGroup is the type of check driver, which v2 named "om".
	DriverGroup = "om"
	// DriverName is the name of check driver.
	DriverName = "openmanage"
)

type (
	checker struct{}
)

var dirs = []string{"/opt/dell/srvadmin/bin"}

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	for _, command := range []string{"system", "chassis"} {
		b, err := checkexec.OutputIn(ctx, "", dirs, "omreport", command)
		if errors.Is(err, checkexec.ErrNotFound) {
			return rs, nil
		} else if err != nil {
			continue
		}
		for _, r := range results(b) {
			r.DriverGroup = DriverGroup
			r.DriverName = DriverName
			rs.Push(r)
		}
	}
	return rs, nil
}

// results reads the "<state> : <component>" lines of an omreport output.
func results(b []byte) []check.Result {
	var l []check.Result
	for _, line := range strings.Split(string(b), "\n") {
		parts := strings.Split(line, " : ")
		if len(parts) != 2 {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(parts[0]))
		if state == "severity" {
			continue
		}
		var v int64
		if state != "ok" {
			v = 1
		}
		l = append(l, check.Result{Instance: strings.ToLower(strings.TrimSpace(parts[1])), Value: v})
	}
	return l
}
