package chkraid

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
)

type (
	sas2ircu struct{}
)

var sas2ircuDirs = []string{"/usr/local/admin"}

func (t *sas2ircu) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	if checkexec.Find("sas2ircu", sas2ircuDirs...) == "" {
		return rs, nil
	}
	// sas2ircu writes its log in its working directory.
	dir, err := os.MkdirTemp("", "check.sas2ircu.")
	if err != nil {
		return rs, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	b, err := checkexec.OutputIn(ctx, dir, sas2ircuDirs, "sas2ircu", "LIST")
	if errors.Is(err, checkexec.ErrNotFound) {
		return rs, nil
	} else if err != nil {
		return rs, err
	}
	var errs int64
	for _, ix := range controllers(b) {
		b, err := checkexec.OutputIn(ctx, dir, sas2ircuDirs, "sas2ircu", ix, "DISPLAY")
		if err != nil {
			continue
		}
		for _, r := range parseDisplay(ix, b) {
			errs += r.Value
			r.DriverGroup = DriverGroup
			r.DriverName = "sas2ircu"
			rs.Push(r)
		}
	}
	rs.Push(check.Result{DriverGroup: DriverGroup, DriverName: "sas2ircu", Instance: "all SAS20*", Value: errs})
	return rs, nil
}

// controllers returns the indexes of the SAS2 controllers a sas2ircu LIST
// output lists, as "  0     SAS2008     1000h    72h ...": the SAS2008,
// 2108, 2208 and 2308 sas2ircu drives.
func controllers(b []byte) []string {
	var l []string
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasPrefix(fields[1], "SAS2") {
			continue
		}
		if _, err := strconv.Atoi(fields[0]); err == nil {
			l = append(l, fields[0])
		}
	}
	return l
}

// parseDisplay reads the state of the volumes, the disks and the enclosures
// of a sas2ircu DISPLAY output, 1 when not okay, optimal or standby.
func parseDisplay(ix string, b []byte) []check.Result {
	const (
		none = iota
		disk
		volume
		enclosure
	)
	var (
		l       []check.Result
		section = none
		slot    string
		enc     string
		ctrl    = "ctrl:" + ix
	)
	last := func(line string) string {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return ""
		}
		return fields[len(fields)-1]
	}
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(line, "IR volume"):
			section = volume
		case strings.HasPrefix(line, "Device is a Hard disk"):
			section = disk
		case strings.HasPrefix(line, "Device is a Enclosure services device"):
			section = enclosure
		case section == volume && strings.HasPrefix(line, "  Volume Name") && strings.Contains(line, "Virtual Disk"):
			slot = "LD" + last(line)
		case section == volume && strings.HasPrefix(line, "  Status of volume"):
			l = append(l, check.Result{Instance: ctrl + "," + slot, Value: boolValue(!strings.Contains(line, "Okay (OKY)"))})
		case section == disk && strings.HasPrefix(line, "  Enclosure #"):
			enc = last(line)
		case section == disk && strings.HasPrefix(line, "  Slot #"):
			slot = "PD" + enc + ":" + last(line)
		case section == disk && strings.HasPrefix(line, "  State"):
			l = append(l, check.Result{Instance: ctrl + "," + slot, Value: boolValue(!strings.Contains(line, "Optimal (OPT)"))})
		case section == enclosure && strings.HasPrefix(line, "  Enclosure #"):
			slot = "Enc" + last(line)
		case section == enclosure && strings.HasPrefix(line, "  State"):
			l = append(l, check.Result{Instance: ctrl + "," + slot, Value: boolValue(!strings.Contains(line, "Standby (SBY)"))})
		}
	}
	return l
}
