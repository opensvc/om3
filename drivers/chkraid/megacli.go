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
	megacli struct{}
)

var megacliDirs = []string{"/usr/local/admin", "/opt/MegaRAID/MegaCli"}

func (t *megacli) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	name := ""
	for _, s := range []string{"MegaCli64", "MegaCli", "megacli"} {
		if checkexec.Find(s, megacliDirs...) != "" {
			name = s
			break
		}
	}
	if name == "" {
		return rs, nil
	}
	// MegaCli writes its logs in its working directory.
	dir, err := os.MkdirTemp("", "check.megacli.")
	if err != nil {
		return rs, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	for _, c := range []struct {
		args  []string
		parse func([]byte) []check.Result
	}{
		{[]string{"-LdPdInfo", "-aALL"}, parseLdPdInfo},
		{[]string{"-AdpBbuCmd", "-GetBbuStatus", "-aALL"}, parseBbuStatus},
	} {
		b, err := checkexec.OutputIn(ctx, dir, megacliDirs, name, c.args...)
		if errors.Is(err, checkexec.ErrNotFound) {
			return rs, nil
		} else if err != nil {
			continue
		}
		for _, r := range c.parse(b) {
			r.DriverGroup = DriverGroup
			r.DriverName = "megacli"
			rs.Push(r)
		}
	}
	return rs, nil
}

// parseLdPdInfo counts, per adapter, the logical drives not optimal and the
// physical drives not online.
func parseLdPdInfo(b []byte) []check.Result {
	var (
		l    []check.Result
		slot string
		errs int64
	)
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Adapter") {
			if slot != "" {
				l = append(l, check.Result{Instance: slot, Value: errs})
			}
			parts := strings.Split(line, "#")
			slot = "slot" + strings.TrimSpace(parts[len(parts)-1])
			errs = 0
		}
		if (strings.HasPrefix(line, "State:") && !strings.Contains(line, "Optimal")) ||
			(strings.HasPrefix(line, "Firmware state:") && !strings.Contains(line, "Online")) {
			errs++
		}
	}
	if slot != "" {
		l = append(l, check.Result{Instance: slot, Value: errs})
	}
	return l
}

// parseBbuStatus reads, per adapter, the presence, charge, temperature and
// health of the battery.
func parseBbuStatus(b []byte) []check.Result {
	var (
		l    []check.Result
		slot string
	)
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(strings.TrimSuffix(line, "%"))
		switch {
		case strings.Contains(line, "Adapter:"):
			slot = "slot" + fields[len(fields)-1]
		case strings.HasPrefix(line, "BatteryType:") && strings.Contains(line, "No Battery"):
			l = append(l, check.Result{Instance: slot + " battery NoBattery", Value: 1})
		case strings.HasPrefix(line, "Relative State of Charge:"):
			if v, err := strconv.ParseInt(fields[len(fields)-1], 10, 64); err == nil {
				l = append(l, check.Result{Instance: slot + " battery charge", Value: v, Unit: "%"})
			}
		case strings.HasPrefix(line, "Temperature:") && len(fields) >= 2:
			if v, err := strconv.ParseInt(fields[len(fields)-2], 10, 64); err == nil {
				l = append(l, check.Result{Instance: slot + " battery temp", Value: v, Unit: "C"})
			}
		case strings.HasPrefix(line, "isSOHGood:"):
			l = append(l, check.Result{Instance: slot + " battery isSOHGood", Value: boolValue(!strings.Contains(line, "Yes"))})
		}
	}
	return l
}
