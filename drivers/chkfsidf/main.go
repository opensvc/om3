package chkfsidf

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkdf"
	"github.com/opensvc/om3/v3/util/df"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "fs_i"
	// DriverName is the name of check driver.
	DriverName = "df"
)

type (
	fsChecker struct{}
)

func init() {
	check.Register(&fsChecker{})
}

func (t *fsChecker) Entries(ctx context.Context) ([]df.Entry, error) {
	return df.Inode(ctx)
}

func (t *fsChecker) ResultSet(ctx context.Context, entry *df.Entry, objs []interface{}) *check.ResultSet {
	path := check.ObjectPathClaimingDir(ctx, entry.MountPoint, objs)
	rs := check.NewResultSet()
	rs.Push(check.Result{
		Instance:    entry.MountPoint,
		Value:       entry.UsedPercent,
		Path:        path,
		Unit:        "%",
		DriverGroup: DriverGroup,
		DriverName:  DriverName,
	})
	rs.Push(check.Result{
		Instance:    entry.MountPoint + ".free",
		Value:       entry.Free,
		Path:        path,
		Unit:        "inode",
		DriverGroup: DriverGroup,
		DriverName:  DriverName,
	})
	rs.Push(check.Result{
		Instance:    entry.MountPoint + ".size",
		Value:       entry.Total,
		Path:        path,
		Unit:        "inode",
		DriverGroup: DriverGroup,
		DriverName:  DriverName,
	})
	return rs
}

func (t *fsChecker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	return checkdf.Check(ctx, t, objs)
}

func main() {
	checker := &fsChecker{}
	if err := check.Check(context.Background(), checker, []interface{}{}); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
