package main

import (
	"fmt"
	"opensvc.com/opensvc/core/check"
	"opensvc.com/opensvc/core/check/helpers/checkdf"
	"opensvc.com/opensvc/util/df"
	"os"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "fs_i"
	// DriverName is the name of check driver.
	DriverName = "df"
)

type fsChecker struct{}

func (t *fsChecker) Entries() ([]df.Entry, error) {
	return df.Inode()
}

// ObjectPath returns the path of the first object using the mount point
// passed as argument
func (t *fsChecker) objectPath(_ string) string {
	return ""
}

func (t *fsChecker) Results(entry *df.Entry) []*check.Result {
	results := make([]*check.Result, 0)
	path := t.objectPath(entry.MountPoint)
	results = append(results, &check.Result{
		Instance:    entry.MountPoint,
		Value:       entry.UsedPercent,
		Path:        path,
		Unit:        "%",
		DriverGroup: DriverGroup,
		DriverName:  DriverName,
	})
	results = append(results, &check.Result{
		Instance:    entry.MountPoint + ".free",
		Value:       entry.Free,
		Path:        path,
		Unit:        "inode",
		DriverGroup: DriverGroup,
		DriverName:  DriverName,
	})
	results = append(results, &check.Result{
		Instance:    entry.MountPoint + ".size",
		Value:       entry.Total,
		Path:        path,
		Unit:        "inode",
		DriverGroup: DriverGroup,
		DriverName:  DriverName,
	})
	return results
}

func (t *fsChecker) Check() ([]*check.Result, error) {
	return checkdf.Check(t)
}

func main() {
	checker := &fsChecker{}
	if err := check.Check(checker); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
