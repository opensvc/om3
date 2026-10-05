package chkraid

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
)

type (
	smartArray struct{}
)

var smartArrayDirs = []string{"/opt/HPQacucli/sbin"}

// smartArrayClis are the names of the HP Smart Array cli, by generation:
// the commands are the same.
var smartArrayClis = []string{"hpacucli", "hpssacli", "ssacli"}

func (t *smartArray) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	cli := ""
	for _, name := range smartArrayClis {
		if cli = checkexec.Find(name, smartArrayDirs...); cli != "" {
			break
		}
	}
	if cli == "" {
		return rs, nil
	}
	run := func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(ctx, checkexec.Timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, cli, args...)
		cmd.Env = append(os.Environ(), "INFOMGR_BYPASS_NONSA=1")
		return cmd.Output()
	}
	b, err := run("controller", "all", "show", "status")
	if err != nil {
		return rs, err
	}
	for _, slot := range smartArraySlots(b) {
		for _, scope := range [][]string{{"show"}, {"array", "all", "show"}, {"logicaldrive", "all", "show"}, {"physicaldrive", "all", "show"}} {
			args := append(append([]string{"controller", "slot=" + slot}, scope...), "status")
			b, err := run(args...)
			if err != nil {
				continue
			}
			for _, r := range parseSmartArrayStatus(b) {
				r.Instance = "slot " + slot + "." + r.Instance
				r.DriverGroup = DriverGroup
				r.DriverName = "smartarray"
				rs.Push(r)
			}
		}
	}
	return rs, nil
}

// smartArraySlots returns the slots of the controllers a controller all show
// status output lists, as "Smart Array P410i in Slot 0 (Embedded)".
func smartArraySlots(b []byte) []string {
	var l []string
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.Contains(line, " Slot ") {
			continue
		}
		fields := strings.Fields(line)
		if i := slices.Index(fields, "Slot"); i >= 0 && i+1 < len(fields) {
			l = append(l, fields[i+1])
		}
	}
	return l
}

// parseSmartArrayStatus reads the "  <component>: <status>" lines of a show
// status output: a component OK is named and 0, another is named by its
// whole line, status included, and 1.
func parseSmartArrayStatus(b []byte) []check.Result {
	var l []check.Result
	for _, line := range strings.Split(string(b), "\n") {
		parts := strings.Split(line, ": ")
		if len(parts) < 2 || !strings.HasPrefix(line, "  ") {
			continue
		}
		if strings.TrimSpace(parts[len(parts)-1]) == "OK" {
			l = append(l, check.Result{Instance: strings.ToLower(strings.TrimSpace(parts[0])), Value: 0})
		} else {
			l = append(l, check.Result{Instance: strings.ToLower(strings.TrimSpace(line)), Value: 1})
		}
	}
	return l
}
