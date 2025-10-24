package ressync

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/schedule"
)

type (
	T struct {
		resource.T
		MaxDelay *time.Duration `json:"max_delay"`
		Schedule string         `json:"schedule"`
		Path     naming.Path    `json:"path"`
	}
)

var (
	//go:embed text
	fs embed.FS

	KWMaxDelay = keywords.Keyword{
		Aliases:       []string{"sync_max_delay"},
		Attr:          "MaxDelay",
		Converter:     "duration",
		DefaultOption: "sync_max_delay",
		Option:        "max_delay",
		Text:          keywords.NewText(fs, "text/kw/max_delay"),
	}
	KWSchedule = keywords.Keyword{
		Attr:          "Schedule",
		DefaultOption: "sync_schedule",
		Example:       "00:00-01:00 mon",
		Option:        "schedule",
		Scopable:      true,
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
func (t *T) GetMaxDelay(lastSync time.Time) time.Duration {
	if t.MaxDelay != nil {
		return *t.MaxDelay
	}
	sched := schedule.New(t.Schedule)
	begin, duration, err := sched.Next(schedule.NextWithLast(lastSync))
	if err != nil {
		return 0
	}
	end := begin.Add(duration)
	maxDelay := end.Sub(time.Now())
	if maxDelay < 0 {
		return 0
	}
	return maxDelay
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
			if maxDelay == 0 {
				t.StatusLog().Info("no schedule and no max delay")
				continue
			}
			age := time.Since(tm)
			if age > maxDelay {
				t.StatusLog().Warn("%s last sync is too old, at %s (>%s ago)", nodename, tm, maxDelay)
				state.Add(status.Warn)
			} else {
				state.Add(status.Up)
			}
		}
	}
	return state
}

func (t *T) WritePeerLastSync(peer string, peers []string) error {
	head := t.GetObjectDriver().VarDir()
	lastSyncFile := t.lastSyncFile(peer)
	lastSyncFileSrc := t.lastSyncFile(hostname.Hostname())
	schedTimestampFile := filepath.Join(head, "scheduler", "last_sync_update_"+t.RID())
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

	c, err := client.New(client.WithURL(peer))
	if err != nil {
		return err
	}

	send := func(filename, nodename string) error {
		file, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer file.Close()
		ctx := context.Background()
		response, err := c.PostInstanceStateFileWithBody(ctx, nodename, t.Path.Namespace, t.Path.Kind, t.Path.Name, "application/octet-stream", file, func(ctx context.Context, req *http.Request) error {
			req.Header.Add(api.HeaderRelativePath, filename[len(head):])
			return nil
		})
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusNoContent {
			return fmt.Errorf("unexpected response: %s", response.Status)
		}
		return nil
	}

	var errs error

	for _, nodename := range peers {
		for _, filename := range []string{lastSyncFile, lastSyncFileSrc, schedTimestampFile} {
			if err := send(filename, nodename); err != nil {
				errs = errors.Join(errs, fmt.Errorf("failed to send state file %s to node %s: %w", filename, nodename, err))
			}
			t.Log().Infof("state file %s sent to node %s", filename, nodename)
		}
	}

	return errs
}

func (t *T) WriteLastSync(nodename string) error {
	p := t.lastSyncFile(nodename)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

func (t *T) readLastSync(nodename string) (time.Time, error) {
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

func (t *T) lastSyncFile(nodename string) string {
	return filepath.Join(t.VarDir(), "last_sync_"+nodename)
}

func (t *T) GetTargetPeernames(target, nodes, drpNodes []string) []string {
	nodenames := make([]string, 0)
	localhost := hostname.Hostname()
	withLocal := slices.Contains(target, "local")
	for _, nodename := range t.GetTargetNodenames(target, nodes, drpNodes) {
		if nodename != localhost {
			nodenames = append(nodenames, nodename)
		} else if withLocal {
			nodenames = append(nodenames, nodename)
		}
	}
	return nodenames
}

func (t *T) GetTargetNodenames(target, nodes, drpNodes []string) []string {
	nodenames := make([]string, 0)
	targetMap := make(map[string]bool)
	for _, t := range target {
		targetMap[t] = false
	}
	if done, ok := targetMap["local"]; ok && !done {
		nodenames = append(nodenames, hostname.Hostname())
		targetMap["local"] = true
	}
	if done, ok := targetMap["nodes"]; ok && !done {
		nodenames = append(nodenames, nodes...)
		targetMap["nodes"] = true
	}
	if done, ok := targetMap["drpnodes"]; ok && !done {
		nodenames = append(nodenames, drpNodes...)
		targetMap["drpnodes"] = true
	}
	return nodenames
}

func (t *T) IsInstanceSufficientlyStarted(ctx context.Context) (v bool, rids []string) {
	sb := statusbus.FromContext(ctx)
	o := t.GetObjectDriver()
	l := o.ResourcesByDrivergroups([]driver.Group{
		driver.GroupIP,
		driver.GroupFS,
		driver.GroupShare,
		driver.GroupDisk,
		driver.GroupContainer,
	})
	v = true
	for _, r := range l {
		switch r.ID().DriverGroup() {
		case driver.GroupIP:
		case driver.GroupFS:
		case driver.GroupShare:
		case driver.GroupDisk:
			switch r.Manifest().DriverID.Name {
			case "drbd":
				continue
			case "scsireserv":
				continue
			}
		case driver.GroupContainer:
		default:
			continue
		}
		st := sb.Get(r.RID())
		switch st {
		case status.Up:
		case status.StandbyUp:
		case status.NotApplicable:
		default:
			// required resource is not up
			rids = append(rids, fmt.Sprintf("%s:%s", r.RID(), st))
			v = false
		}
	}
	return
}
