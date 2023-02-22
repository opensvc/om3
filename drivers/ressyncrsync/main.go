package ressyncrsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/ressync"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/proc"
	"github.com/opensvc/om3/util/schedule"
)

const (
	rsync = "rsync"
)

// T is the driver structure.
type T struct {
	ressync.T
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
	DRPNodes       []string
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
		// DO
		if err := t.writeLastSync(nodename); err != nil {
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

// maxDelay return the configured max_delay if set.
// If not set, return the duration from now to the end of the
// next schedule period.
func (t *T) maxDelay(lastSync time.Time) *time.Duration {
	if t.MaxDelay != nil {
		return t.MaxDelay
	}
	sched := schedule.New(t.Schedule)
	begin, duration, err := sched.Next(schedule.NextWithLast(lastSync))
	if err != nil {
		return nil
	}
	end := begin.Add(duration)
	maxDelay := end.Sub(time.Now())
	if maxDelay < 0 {
		maxDelay = 0
	}
	return &maxDelay
}

func (t *T) Status(ctx context.Context) status.T {
	s := t.statusLastSync()
	return s
}

func (t T) writeLastSync(nodename string) error {
	p := t.lastSyncFile(nodename)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

func (t T) readLastSync(nodename string) (time.Time, error) {
	var tm time.Time
	p := t.lastSyncFile(nodename)
	info, err := os.Stat(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return tm, nil
	case err != nil:
		return tm, err
	default:
		return info.ModTime(), nil
	}
}

func (t T) lastSyncFile(nodename string) string {
	return filepath.Join(t.VarDir(), "last_sync_"+nodename)
}

func (t *T) statusLastSync() status.T {
	nodenames := t.getTargetNodenames()
	if len(nodenames) == 0 {
		t.StatusLog().Info("no target nodes")
		return status.NotApplicable
	}
	state := status.NotApplicable
	for _, nodename := range t.getTargetNodenames() {
		if nodename == hostname.Hostname() {
			continue
		}
		if tm, err := t.readLastSync(nodename); err != nil {
			t.StatusLog().Error("%s last sync: %s", nodename, err)
		} else if tm.IsZero() {
			t.StatusLog().Warn("%s never synced", nodename)
		} else {
			maxDelay := t.maxDelay(tm)
			if maxDelay == nil {
				t.StatusLog().Info("no schedule and no max delay")
				continue
			}
			elapsed := time.Now().Sub(tm)
			if elapsed > *maxDelay {
				t.StatusLog().Warn("%s last sync at %s (>%s after last)", nodename, tm, maxDelay)
				state.Add(status.Warn)
			} else {
				state.Add(status.Up)
			}
		}
	}
	return state
}

func (t *T) getTargetNodenames() []string {
	nodenames := make([]string, 0)
	targetMap := make(map[string]any)
	for _, target := range t.Target {
		targetMap[target] = nil
	}
	if _, ok := targetMap["nodes"]; ok {
		nodenames = append(nodenames, t.Nodes...)
	}
	if _, ok := targetMap["drpnodes"]; ok {
		nodenames = append(nodenames, t.DRPNodes...)
	}
	return nodenames
}

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

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}
