// Package chkmcelog is the mcelog check driver: the number of machine check
// lines logged in /var/log/mcelog since the previous check.
//
// Each check appends a marker line to the log, and the next one counts the
// lines after it: all of them when the marker is not found, as after a log
// rotation.
package chkmcelog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "mcelog"
	// DriverName is the name of check driver.
	DriverName = "mcelog"
)

type (
	checker struct{}
)

var logPath = "/var/log/mcelog"

func init() {
	check.Register(&checker{})
}

func markerPath() string {
	return filepath.Join(rawconfig.Paths.Tmp, "check.mcelog.marker")
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	if checkexec.Find("mcelog", "/sbin", "/usr/sbin") == "" {
		return rs, nil
	}
	b, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return rs, nil
	} else if err != nil {
		return rs, err
	}
	marker, _ := os.ReadFile(markerPath())
	rs.Push(check.Result{
		DriverGroup: DriverGroup,
		DriverName:  DriverName,
		Instance:    "new lines",
		Value:       newLines(b, strings.TrimSuffix(string(marker), "\n")),
	})
	if err := appendMarker(); err != nil {
		return rs, err
	}
	return rs, nil
}

// newLines counts the lines of the log after the marker, all of them when
// the marker is not found.
func newLines(b []byte, marker string) int64 {
	var total, after int64
	found := false
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if line == "" && total == 0 {
			continue
		}
		total++
		if marker != "" && line == marker {
			found = true
			after = 0
			continue
		}
		after++
	}
	if !found {
		return total
	}
	return after
}

// appendMarker appends a new marker line to the log, and records it.
func appendMarker() error {
	marker := fmt.Sprintf("opensvc marker %s\n", time.Now().Format(time.RFC3339Nano))
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(marker); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(markerPath()), 0o700); err != nil {
		return err
	}
	return os.WriteFile(markerPath(), []byte(marker), 0o600)
}
