package restask

import (
	"context"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resourceid"
)

// installer is the object installing its resources for a task.
type installer interface {
	InstallResources(ctx context.Context, rids []string) error
}

// validInstall refuses an install value naming a resource installing no
// file: a volume or a fs.
func validInstall(v string) error {
	for _, s := range strings.Fields(v) {
		rid, err := resourceid.Parse(s)
		if err != nil {
			return fmt.Errorf("install %s: %w", v, err)
		}
		switch rid.DriverGroup() {
		case driver.GroupFS, driver.GroupVolume:
		default:
			return fmt.Errorf("install %s: %s is not a fs or volume resource", v, s)
		}
	}
	return nil
}

// RunInstall installs again the files of the resources the install keyword
// names, rendering their templates from what the node knows now, before the
// task does its own work. The files it changes are recorded, which the daemon
// tells the tasks on FileInstalled.
//
// A failed install is the failed run of the task.
func (t *BaseTask) RunInstall(ctx context.Context) error {
	if len(t.Install) == 0 {
		return nil
	}
	err := t.runInstall(ctx)
	if err != nil {
		if err := t.WriteLastRun(1); err != nil {
			t.Log().Errorf("write last run: %s", err)
		}
	}
	return err
}

func (t *BaseTask) runInstall(ctx context.Context) error {
	i, ok := t.GetObjectDriver().(installer)
	if !ok {
		return fmt.Errorf("install %s: the object installs no resource", strings.Join(t.Install, " "))
	}
	t.Log().Infof("install %s", strings.Join(t.Install, " "))
	return i.InstallResources(ctx, t.Install)
}
