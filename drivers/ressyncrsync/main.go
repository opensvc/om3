package ressyncrsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/proc"
)

const (
	rsync = "rsync"
)

// T is the driver structure.
type T struct {
	resource.T
	Path           path.T
	BandwidthLimit string
	Src            string
	Dst            string
	DstFS          string
	Options        []string
	Target         []string
	Schedule       string
	ResetOptions   bool
	Snap           bool
	Snooze         *time.Duration
	Nodes          []string
	ObjectID       uuid.UUID
	Timeout        *time.Duration
}

func New() resource.Driver {
	return &T{}
}

func (t T) IsRunning() bool {
	err := t.DoWithLock(false, time.Second*0, "run", func() error {
		return nil
	})
	return err != nil
}

// Start the Resource
func (t T) Start(ctx context.Context) (err error) {
	return nil
}

func (t T) Sync(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	err := t.DoWithLock(disable, timeout, "run", func() error {
		return t.lockedSync(ctx)
	})
	return err
}

func (t T) lockedSync(ctx context.Context) (err error) {
	for _, nodename := range t.Nodes {
		if nodename == hostname.Hostname() {
			continue
		}
		exitCode := 0
		// DO
		if err := t.writeLastSync(nodename, exitCode); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Kill(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	return status.NotApplicable
}

func (t T) writeLastSync(nodename string, retcode int) error {
	p := t.lastSyncFile(nodename)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintf(f, "%d\n", retcode)
	return nil
}

func (t T) readLastSync(nodename string) (int, error) {
	p := t.lastSyncFile(nodename)
	if b, err := os.ReadFile(p); err != nil {
		return 0, err
	} else {
		return strconv.Atoi(strings.TrimSpace(string(b)))
	}
}

func (t T) lastSyncFile(nodename string) string {
	return filepath.Join(t.VarDir(), "last_sync_"+nodename)
}

/*
func (t *T) statusLastSync(ctx context.Context) status.T {
	if err := resource.StatusCheckRequires(ctx, t); err != nil {
		t.StatusLog().Info("requirements not met")
		return status.NotApplicable
	}
	if i, err := t.readLastRun(); err != nil {
		t.StatusLog().Info("never run")
		return status.NotApplicable
	} else {
		s, err := t.ExitCodeToStatus(i)
		if err != nil {
			t.StatusLog().Info("%s", err)
		}
		if s != status.Up {
			t.StatusLog().Info("last run failed (%d)", i)
		}
		return s
	}
}
*/

func (t *T) running(ctx context.Context) bool {
	return false
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	switch {
	case t.Src != "" && len(t.Target) > 0:
		return t.Src + " to " + strings.Join(t.Target, " ")
	case t.Src != "":
		return t.Src + " to void"
	case len(t.Target) > 0:
		return "nothing to " + strings.Join(t.Target, " ")
	default:
		return ""
	}
}

func (t *T) status() status.T {
	return status.Undef
}

func (t T) getRunning(cmdArgs []string) (proc.L, error) {
	procs, err := proc.All()
	if err != nil {
		return procs, err
	}
	procs = procs.FilterByEnv("OPENSVC_ID", t.ObjectID.String())
	procs = procs.FilterByEnv("OPENSVC_RID", t.RID())
	return procs, nil
}

func (t T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action: "sync",
		Option: "schedule",
		Base:   "",
	}
}
