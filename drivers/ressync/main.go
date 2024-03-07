package ressync

import (
	"context"
	"embed"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/schedule"
)

type (
	T struct {
		resource.T
		MaxDelay *time.Duration
		Schedule string
	}
)

var (
	//go:embed text
	fs embed.FS

	KWMaxDelay = keywords.Keyword{
		Option:        "max_delay",
		DefaultOption: "sync_max_delay",
		Aliases:       []string{"sync_max_delay"},
		Attr:          "MaxDelay",
		Converter:     converters.Duration,
		Text:          keywords.NewText(fs, "text/kw/max_delay"),
	}
	KWSchedule = keywords.Keyword{
		Option:        "schedule",
		DefaultOption: "sync_schedule",
		Attr:          "Schedule",
		Scopable:      true,
		Example:       "00:00-01:00 mon",
		Text:          keywords.NewText(fs, "text/kw/schedule"),
	}

	BaseKeywords = append(
		[]keywords.Keyword{},
		KWMaxDelay,
		KWSchedule,
	)
)

// GetMaxDelay return the configured max_delay if set.
// If not set, return the duration from now to the end of the
// next schedule period.
func (t *T) GetMaxDelay(lastSync time.Time) *time.Duration {
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

func (t *T) StatusLastSync(nodenames []string) status.T {
	state := status.NotApplicable

	if len(nodenames) == 0 {
		t.StatusLog().Info("no target nodes")
		return status.NotApplicable
	}

	for _, nodename := range nodenames {
		if tm, err := t.readLastSync(nodename); err != nil {
			t.StatusLog().Error("%s last sync: %s", nodename, err)
		} else if tm.IsZero() {
			t.StatusLog().Warn("%s never synced", nodename)
		} else {
			maxDelay := t.GetMaxDelay(tm)
			if maxDelay == nil || *maxDelay == 0 {
				t.StatusLog().Info("no schedule and no max delay")
				continue
			}
			elapsed := time.Now().Sub(tm)
			if elapsed > *maxDelay {
				t.StatusLog().Warn("%s last sync at %s (>%s after last)", nodename, tm, maxDelay)
				state.Add(status.Warn)
			} else {
				//t.StatusLog().Info("%s last sync at %s (%s after last)", nodename, tm, maxDelay)
				state.Add(status.Up)
			}
		}
	}
	return state
}

func (t T) WritePeerLastSync(nodename, user string) error {
	lastSyncFile := t.lastSyncFile(nodename)
	lastSyncFileSrc := t.lastSyncFile(hostname.Hostname())
	schedTimestampFile := filepath.Join(t.GetObjectDriver().VarDir(), "scheduler", "last_sync_update_"+t.RID())
	now := time.Now()
	if err := file.Touch(lastSyncFile, now); err != nil {
		return err
	}
	if err := file.Touch(lastSyncFileSrc, now); err != nil {
		return err
	}
	if err := file.Touch(schedTimestampFile, now); err != nil {
		return err
	}
	dst := user + "@" + nodename + ":/"
	args := make([]string, 0)
	args = append(args, "-R", lastSyncFile, lastSyncFileSrc, schedTimestampFile, dst)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmdStr := cmd.String()
	t.Log().Attr("cmd", cmdStr).Infof("copy state file to node %s", nodename)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Log().
			Attr("cmd", cmdStr).
			Attr("outputs", string(b)).
			Errorf("copy state file to node %s: %s", nodename, err)
		return err
	}
	return nil
}

func (t T) WriteLastSync(nodename string) error {
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

func (t *T) GetTargetPeernames(target, nodes, drpNodes []string) []string {
	nodenames := make([]string, 0)
	localhost := hostname.Hostname()
	for _, nodename := range t.GetTargetNodenames(target, nodes, drpNodes) {
		if nodename != localhost {
			nodenames = append(nodenames, nodename)
		}
	}
	return nodenames
}

func (t *T) GetTargetNodenames(target, nodes, drpNodes []string) []string {
	nodenames := make([]string, 0)
	targetMap := make(map[string]any)
	for _, target := range target {
		targetMap[target] = nil
	}
	if _, ok := targetMap["nodes"]; ok {
		nodenames = append(nodenames, nodes...)
	}
	if _, ok := targetMap["drpnodes"]; ok {
		nodenames = append(nodenames, drpNodes...)
	}
	return nodenames
}
